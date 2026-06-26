package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pgevidence/internal/archive"
	"pgevidence/internal/capture"
	"pgevidence/internal/config"
	"pgevidence/internal/manifest"
	"pgevidence/internal/psql"
	"pgevidence/internal/runner"
	"pgevidence/internal/store"
)

// App is the Wails-bound application object. Its exported methods are callable
// from the frontend; it also implements runner.UI.
type App struct {
	ctx       context.Context
	mu        sync.Mutex
	passwords map[string]string // session passwords, in memory only, keyed by connection ID
	running   bool
	cancel    context.CancelFunc
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{passwords: map[string]string{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if cfg, err := config.Load(); err == nil {
		applyToolPaths(cfg)
	}
}

// applyToolPaths pushes the configured psql/ffmpeg overrides into the resolvers.
func applyToolPaths(cfg config.Config) {
	psql.SetPath(cfg.PsqlPath)
	capture.SetFFmpegPath(cfg.FfmpegPath)
}

// --- runner.UI implementation -------------------------------------------------

// Emit forwards an event to the frontend.
func (a *App) Emit(event string, data interface{}) {
	wruntime.EventsEmit(a.ctx, event, data)
}

// BringToFront makes the window visible so it appears in screenshots.
func (a *App) BringToFront() {
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
}

// --- configuration ------------------------------------------------------------

func (a *App) GetConfig() (config.Config, error) { return config.Load() }

func (a *App) SaveConfig(cfg config.Config) error {
	applyToolPaths(cfg)
	// Window size is owned by SaveWindowSize and never sent by the Settings UI, so
	// preserve whatever is on disk — otherwise a Settings save (carrying a stale
	// window size) would clobber a size persisted by a resize.
	return config.Update(func(cur *config.Config) {
		w, h := cur.WindowWidth, cur.WindowHeight
		*cur = cfg
		cur.WindowWidth, cur.WindowHeight = w, h
	})
}

// UpdateTheme persists only the theme field without disturbing other settings.
func (a *App) UpdateTheme(theme string) error {
	return config.Update(func(cur *config.Config) { cur.Theme = theme })
}

// --- query management ---------------------------------------------------------

func (a *App) ListQueries() ([]store.Query, error)              { return store.List() }
func (a *App) SaveQuery(q store.Query) ([]store.Query, error)   { return store.Upsert(q) }
func (a *App) DeleteQuery(id string) ([]store.Query, error)     { return store.Delete(id) }
func (a *App) MoveQuery(id string, d int) ([]store.Query, error) { return store.Move(id, d) }
func (a *App) ReplaceAllQueries(q []store.Query) ([]store.Query, error) {
	return store.ReplaceAll(q)
}
func (a *App) ImportQueries(text string) ([]store.Query, error) { return store.Import(text) }
func (a *App) ExportQueries() (string, error)                   { return store.Export() }

// --- session credentials (in memory only) -------------------------------------

func (a *App) SetSessionPassword(connID, pw string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if pw == "" {
		delete(a.passwords, connID)
		return
	}
	a.passwords[connID] = pw
}

func (a *App) HasSessionPassword(connID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.passwords[connID]
	return ok
}

func (a *App) ClearSessionPasswords() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.passwords = map[string]string{}
}

func (a *App) password(connID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.passwords[connID]
}

// --- environment / diagnostics ------------------------------------------------

// EnvInfo describes the host environment relevant to a run.
type EnvInfo struct {
	PSQLFound   bool   `json:"psqlFound"`
	PSQLPath    string `json:"psqlPath"`
	PSQLVersion string `json:"psqlVersion"`
	FFmpegFound  bool   `json:"ffmpegFound"`
	NumDisplays  int    `json:"numDisplays"`
	ConfigDir    string `json:"configDir"`
	ScreenAccess bool   `json:"screenAccess"`
	AppVersion   string `json:"appVersion"`
	AppName      string `json:"appName"`
}

func (a *App) DetectEnvironment() EnvInfo {
	if cfg, err := config.Load(); err == nil {
		applyToolPaths(cfg) // reflect any custom psql/ffmpeg path in detection
	}
	info := EnvInfo{
		FFmpegFound:  capture.FFmpegAvailable(),
		NumDisplays:  capture.NumDisplays(),
		ScreenAccess: capture.HasScreenAccess(),
		AppVersion:   AppVersion,
		AppName:      AppName,
	}
	if path, ver, err := psql.Detect(); err == nil {
		info.PSQLFound = true
		info.PSQLPath = path
		info.PSQLVersion = ver
	}
	if dir, err := config.AppDir(); err == nil {
		info.ConfigDir = dir
	}
	return info
}

// RequestScreenAccess prompts for macOS Screen Recording permission and returns
// whether it is granted. The app usually needs a restart after granting.
func (a *App) RequestScreenAccess() bool {
	return capture.RequestScreenAccess()
}

// OpenScreenRecordingSettings opens the macOS Screen Recording privacy pane.
// CGRequestScreenCaptureAccess only ever prompts once; once the app is listed
// (even if the user disabled it), re-requesting is a no-op — so to actually
// change the grant the user must toggle it here.
func (a *App) OpenScreenRecordingSettings() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return exec.Command("open",
		"x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture").Start()
}

// GrantScreenAccess does ONE thing: a real capture attempt that registers the app
// in the Screen Recording list. It is self-regulating — macOS shows the permission
// prompt only when status is "not determined" (first ask) and is silent when
// access was denied. It never opens Settings (the UI offers that as a separate
// action), so the user never gets both the prompt and Settings at once. There is
// no public API to query list membership, so we register unconditionally rather
// than try to detect it.
func (a *App) GrantScreenAccess() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	capture.RegisterForScreenAccess()
	return nil
}

// TestConnection runs `SELECT 1` against the given connection.
func (a *App) TestConnection(connID string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applyToolPaths(cfg)
	c, ok := cfg.FindConnection(connID)
	if !ok {
		return fmt.Errorf("connection %q not found", connID)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return psql.Test(ctx, toPsqlConn(c), a.password(connID))
}

// --- run control --------------------------------------------------------------

func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// StartRun kicks off the run on a background goroutine and returns immediately.
// Progress is delivered through run:* events.
// StartRun runs with per-run overrides from the Run page (screenshots, video,
// connection) layered over the saved config — WITHOUT persisting them.
func (a *App) StartRun(screenshots bool, video bool, connectionID string) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("a run is already in progress")
	}
	a.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applyToolPaths(cfg)
	// Per-run overrides (not saved): the Run page is the authority for these.
	cfg.Screenshots = screenshots
	cfg.Video = video
	if connectionID == "" {
		connectionID = cfg.SelectedConnectionID
	}
	c, ok := cfg.FindConnection(connectionID)
	if !ok {
		return fmt.Errorf("no connection selected")
	}
	all, err := store.List()
	if err != nil {
		return err
	}
	var enabled []store.Query
	for _, q := range all {
		if q.Enabled {
			enabled = append(enabled, q)
		}
	}
	if len(enabled) == 0 {
		return fmt.Errorf("no enabled queries to run")
	}
	_, psqlVer, derr := psql.Detect()
	if derr != nil {
		return derr
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.running = true
	a.cancel = cancel
	a.mu.Unlock()

	params := runner.Params{
		Cfg:         cfg,
		Conn:        toPsqlConn(c),
		ConnInfo:    toConnInfo(c),
		Password:    a.password(c.ID),
		Queries:     enabled,
		PSQLVersion: psqlVer,
		AppVersion:  AppVersion,
	}

	go func() {
		defer func() {
			// A panic in the run loop must not take the whole app down: report it
			// as a failed run and always release the running state.
			if r := recover(); r != nil {
				a.Emit(runner.EventDone, map[string]interface{}{
					"error": fmt.Sprintf("internal error during run: %v", r),
				})
			}
			a.mu.Lock()
			a.running = false
			a.cancel = nil
			a.mu.Unlock()
		}()
		if _, err := runner.Run(ctx, a, params); err != nil {
			a.Emit(runner.EventDone, map[string]interface{}{"error": err.Error()})
		}
	}()
	return nil
}

// CancelRun requests cancellation of the active run.
func (a *App) CancelRun() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// SelectOutputDir opens a native folder picker and returns the chosen path.
func (a *App) SelectOutputDir() (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose output folder for audit extracts",
	})
}

// SelectFile opens a native file picker (for choosing a psql/ffmpeg binary).
func (a *App) SelectFile(title string) (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
}

// ArchiveRun zips the run folder into <runDir>/<name>.zip. A non-empty password
// ZipCrypto-encrypts the entries; empty means an unencrypted archive.
func (a *App) ArchiveRun(runDir, password string, excludeVideo bool) (archive.Result, error) {
	zipPath, err := archive.Create(runDir, password, excludeVideo)
	if err != nil {
		return archive.Result{}, err
	}
	mode := "none"
	if password != "" {
		mode = "explicit"
	}
	return archive.Result{ZipPath: zipPath, Encrypted: password != "", Mode: mode}, nil
}

// ArchiveRunAuto zips the run folder with a generated password and writes that
// password to <zip>.pwd next to the archive.
func (a *App) ArchiveRunAuto(runDir string, excludeVideo bool) (archive.Result, error) {
	pw, err := archive.GeneratePassword()
	if err != nil {
		return archive.Result{}, err
	}
	zipPath, err := archive.Create(runDir, pw, excludeVideo)
	if err != nil {
		return archive.Result{}, err
	}
	pwdPath, err := archive.WritePwdFile(zipPath, pw)
	if err != nil {
		return archive.Result{}, err
	}
	return archive.Result{ZipPath: zipPath, PwdPath: pwdPath, Password: pw, Encrypted: true, Mode: "auto"}, nil
}

// PruneRunDir deletes the loose source files in a run folder, keeping only the
// ZIP archive (and its .pwd). Safe: it refuses if the archive is missing/empty.
func (a *App) PruneRunDir(runDir string, keepVideo bool) error {
	return archive.PruneSources(runDir, keepVideo)
}

// SaveWindowSize persists the current OS window size so it can be restored on
// next launch. The frontend passes Wails' WindowGetSize (the OS window size, not
// the webview viewport) — otherwise the window would shrink a little each launch.
func (a *App) SaveWindowSize(width, height int) error {
	return config.Update(func(cur *config.Config) {
		cur.WindowWidth = width
		cur.WindowHeight = height
	})
}

// OpenRunFolder reveals a run directory in the OS file manager.
func (a *App) OpenRunFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// --- helpers ------------------------------------------------------------------

func toPsqlConn(c config.Connection) psql.Conn {
	return psql.Conn{
		Host:    c.Host,
		Port:    c.Port,
		DBName:  c.DBName,
		User:    c.User,
		SSLMode: c.SSLMode,
	}
}

func toConnInfo(c config.Connection) manifest.ConnInfo {
	return manifest.ConnInfo{
		Name:    c.Name,
		Host:    c.Host,
		Port:    c.Port,
		DBName:  c.DBName,
		User:    c.User,
		SSLMode: c.SSLMode,
	}
}
