// Package runner orchestrates an audit run: it executes each enabled query in
// order, writes the CSV result and checksum, shows the query on screen for a
// configurable dwell time, captures a full-screen screenshot, and finally writes
// a tamper-evident manifest.
package runner

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pgevidence/internal/capture"
	"pgevidence/internal/checksum"
	"pgevidence/internal/config"
	"pgevidence/internal/manifest"
	"pgevidence/internal/psql"
	"pgevidence/internal/store"
)

// UI is the surface the runner uses to talk to the frontend. App implements it
// with the Wails runtime.
type UI interface {
	Emit(event string, data interface{})
	BringToFront()
}

// Params bundles everything a run needs.
type Params struct {
	Cfg         config.Config
	Conn        psql.Conn
	ConnInfo    manifest.ConnInfo
	Password    string
	Queries     []store.Query
	PSQLVersion string
	AppVersion  string
}

// Event names emitted to the frontend.
const (
	EventStart  = "run:start"
	EventQuery  = "run:query"
	EventResult = "run:result"
	EventDwell  = "run:dwell"
	EventLog    = "run:log"
	EventDone   = "run:done"
)

// Run executes the whole run and returns the run directory. It honours ctx
// cancellation between and within steps.
func Run(ctx context.Context, ui UI, p Params) (string, error) {
	if len(p.Queries) == 0 {
		return "", fmt.Errorf("no enabled queries to run")
	}
	if err := os.MkdirAll(p.Cfg.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	runDir := filepath.Join(p.Cfg.OutputDir, "audit-run-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create run dir: %w", err)
	}

	ui.BringToFront()
	total := len(p.Queries)
	ui.Emit(EventStart, map[string]interface{}{"runDir": runDir, "total": total})

	// Optional screen recording (best-effort).
	var rec *capture.Recorder
	videoFile := ""
	if p.Cfg.Video {
		if capture.RecordingAvailable() {
			vf := filepath.Join(runDir, "run.mp4")
			r, err := capture.StartRecording(vf, p.Cfg.MonitorIndex)
			if err != nil {
				ui.Emit(EventLog, logMsg("video recording could not start: "+err.Error()))
			} else {
				rec, videoFile = r, vf
			}
		} else {
			ui.Emit(EventLog, logMsg("video requested but no recording backend is available (ffmpeg, or gstreamer on Wayland) — continuing with screenshots only"))
		}
	}

	records := make([]manifest.QueryRecord, 0, total)
	cancelled := false

	for i, q := range p.Queries {
		if ctx.Err() != nil {
			cancelled = true
			break
		}
		idx := i + 1
		stem := fmt.Sprintf("%04d_%s", idx, slug(q.Name))
		resultFile := stem + ".csv"
		resultPath := filepath.Join(runDir, resultFile)

		ui.Emit(EventQuery, map[string]interface{}{
			"index": idx, "total": total, "name": q.Name, "sql": q.SQL,
		})

		rd := manifest.QueryRecord{
			Index:      idx,
			Name:       q.Name,
			SQL:        q.SQL,
			ResultFile: resultFile,
			StartedAt:  time.Now().Format(time.RFC3339),
			Status:     "ok",
		}

		// Optionally store the query itself next to its result (no checksum).
		if p.Cfg.SaveQuerySQL {
			sqlFile := stem + ".sql"
			if werr := os.WriteFile(filepath.Join(runDir, sqlFile), []byte(q.SQL+"\n"), 0o644); werr != nil {
				ui.Emit(EventLog, logMsg("could not write .sql for "+q.Name+": "+werr.Error()))
			} else {
				rd.SQLFile = sqlFile
			}
		}

		res, err := psql.RunToFile(ctx, p.Conn, q.SQL, p.Cfg.EnforceReadOnly, p.Password, resultPath)
		rd.DurationMS = res.Duration.Milliseconds()
		rd.EndedAt = time.Now().Format(time.RFC3339)

		var header []string
		var rows [][]string
		if err != nil {
			rd.Status = "error"
			rd.Error = err.Error()
		} else {
			sum, sumPath, cerr := checksum.WriteSidecar(resultPath)
			if cerr != nil {
				rd.Status = "error"
				rd.Error = "checksum: " + cerr.Error()
			} else {
				rd.SHA256 = sum
				rd.ChecksumFile = filepath.Base(sumPath)
			}
			var perr error
			header, rows, rd.RowCount, perr = previewAndCount(resultPath, p.Cfg.PreviewRows)
			if perr != nil {
				ui.Emit(EventLog, logMsg("preview parse warning for "+q.Name+": "+perr.Error()))
			}
		}

		ui.Emit(EventResult, map[string]interface{}{
			"index": idx, "total": total, "name": q.Name, "sql": q.SQL,
			"sha256": rd.SHA256, "header": header, "rows": rows,
			"rowCount": rd.RowCount, "status": rd.Status, "error": rd.Error,
			"durationMs": rd.DurationMS, "resultFile": resultFile,
		})

		// Let the UI paint the result, then capture the full screen (which
		// includes the OS clock). Only after the capture completes do we start
		// the dwell hold and tell the UI to begin its countdown, so the on-screen
		// timer matches the real remaining time (the settle + capture happen in a
		// brief "capturing" phase before the countdown).
		_ = sleepCtx(ctx, 350*time.Millisecond)
		if p.Cfg.Screenshots {
			shotFile := stem + ".png"
			if serr := capture.ScreenshotContext(ctx, p.Cfg.MonitorIndex, filepath.Join(runDir, shotFile)); serr != nil {
				ui.Emit(EventLog, logMsg("screenshot failed for "+q.Name+": "+serr.Error()))
			} else {
				rd.ScreenshotFile = shotFile
			}
		}
		ui.Emit(EventDwell, map[string]interface{}{"index": idx, "seconds": p.Cfg.DwellSeconds})
		if !sleepCtx(ctx, time.Duration(p.Cfg.DwellSeconds)*time.Second) {
			records = append(records, rd)
			cancelled = true
			break
		}

		records = append(records, rd)
		if rd.Status == "error" && p.Cfg.StopOnError {
			break
		}
	}

	if rec != nil {
		if err := rec.Stop(); err != nil {
			ui.Emit(EventLog, logMsg("stopping video recording: "+err.Error()))
		}
	}

	videoBase := ""
	if videoFile != "" {
		videoBase = filepath.Base(videoFile)
	}
	m := manifest.Manifest{
		AppVersion:   p.AppVersion,
		PSQLVersion:  p.PSQLVersion,
		Connection:   p.ConnInfo,
		GeneratedAt:  time.Now().Format(time.RFC3339),
		RunDir:       runDir,
		DwellSeconds: p.Cfg.DwellSeconds,
		ReadOnly:     p.Cfg.EnforceReadOnly,
		Screenshots:  p.Cfg.Screenshots,
		Video:        videoFile != "",
		VideoFile:    videoBase,
		Queries:      records,
	}
	manifestPath, mErr := manifest.Write(runDir, m)
	if mErr != nil {
		ui.Emit(EventLog, logMsg("writing manifest: "+mErr.Error()))
	}

	ok, failed := 0, 0
	for _, r := range records {
		if r.Status == "ok" {
			ok++
		} else {
			failed++
		}
	}

	ui.Emit(EventDone, map[string]interface{}{
		"runDir": runDir, "manifestFile": filepath.Base(manifestPath),
		"ok": ok, "failed": failed, "cancelled": cancelled,
		"records": records,
	})
	return runDir, nil
}

func logMsg(s string) map[string]interface{} { return map[string]interface{}{"message": s} }

// sleepCtx sleeps for d, returning false if ctx was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// previewAndCount reads the CSV at path once, returning the header row, up to
// max data rows for preview, and the exact total data-row count.
func previewAndCount(path string, max int) (header []string, rows [][]string, count int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.ReuseRecord = true
	first := true
	for {
		rec, rerr := r.Read()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			// Return what we have so far; the file is still valid evidence.
			return header, rows, count, rerr
		}
		if first {
			header = append([]string(nil), rec...)
			first = false
			continue
		}
		count++
		if len(rows) < max {
			rows = append(rows, append([]string(nil), rec...))
		}
	}
	return header, rows, count, nil
}

// slug turns a query name into a filename-safe token.
func slug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('_')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "_")
	}
	if out == "" {
		out = "query"
	}
	return out
}
