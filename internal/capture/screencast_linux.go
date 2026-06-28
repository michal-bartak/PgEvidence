//go:build linux

package capture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const scIface = "org.freedesktop.portal.ScreenCast"

// scSetupTimeout bounds the portal session setup. It's generous because Start
// pops GNOME's "share your screen" picker that the user must confirm.
const scSetupTimeout = 120 * time.Second

var scSeq uint64

// screenCast holds an active xdg-desktop-portal ScreenCast session and the
// PipeWire remote it produced. The D-Bus connection must stay open for the
// lifetime of the recording; closing it ends the session.
type screenCast struct {
	conn          *dbus.Conn
	obj           dbus.BusObject
	sender        string
	sigCh         chan *dbus.Signal
	sessionHandle dbus.ObjectPath
	nodeID        uint32
	fd            int // PipeWire remote fd; -1 once ownership is transferred
}

// startScreenCast negotiates a ScreenCast session via the portal and returns it
// with a PipeWire remote fd + node id ready to feed to a consumer (GStreamer).
func startScreenCast(ctx context.Context) (*screenCast, error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, fmt.Errorf("session bus: %w", err)
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("dbus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("dbus hello: %w", err)
	}
	sc := &screenCast{conn: conn, fd: -1, sender: sanitizeBusName(conn.Names()[0])}
	sc.obj = conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	); err != nil {
		sc.Close()
		return nil, fmt.Errorf("dbus match: %w", err)
	}
	sc.sigCh = make(chan *dbus.Signal, 8)
	conn.Signal(sc.sigCh)

	sessTok := fmt.Sprintf("pgev_sess_%d", atomic.AddUint64(&scSeq, 1))

	// CreateSession
	res, err := sc.request(ctx, "CreateSession", func(tok string) *dbus.Call {
		return sc.obj.CallWithContext(ctx, scIface+".CreateSession", 0, map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(tok),
			"session_handle_token": dbus.MakeVariant(sessTok),
		})
	})
	if err != nil {
		sc.Close()
		return nil, err
	}
	shStr, _ := res["session_handle"].Value().(string)
	if shStr == "" {
		sc.Close()
		return nil, fmt.Errorf("screencast: no session_handle")
	}
	sc.sessionHandle = dbus.ObjectPath(shStr)

	// SelectSources: whole monitor, single, cursor embedded in the video.
	if _, err := sc.request(ctx, "SelectSources", func(tok string) *dbus.Call {
		return sc.obj.CallWithContext(ctx, scIface+".SelectSources", 0, sc.sessionHandle, map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(tok),
			"types":        dbus.MakeVariant(uint32(1)), // 1 = MONITOR
			"multiple":     dbus.MakeVariant(false),
			"cursor_mode":  dbus.MakeVariant(uint32(2)), // 2 = embedded
		})
	}); err != nil {
		sc.Close()
		return nil, err
	}

	// Start: shows the source picker; returns the PipeWire stream(s).
	res, err = sc.request(ctx, "Start", func(tok string) *dbus.Call {
		return sc.obj.CallWithContext(ctx, scIface+".Start", 0, sc.sessionHandle, "", map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(tok),
		})
	})
	if err != nil {
		sc.Close()
		return nil, err
	}
	node, err := firstStreamNode(res)
	if err != nil {
		sc.Close()
		return nil, err
	}
	sc.nodeID = node

	// OpenPipeWireRemote: returns a PipeWire fd (passed over D-Bus).
	var fd dbus.UnixFD
	call := sc.obj.CallWithContext(ctx, scIface+".OpenPipeWireRemote", 0, sc.sessionHandle, map[string]dbus.Variant{})
	if call.Err != nil {
		sc.Close()
		return nil, fmt.Errorf("OpenPipeWireRemote: %w", call.Err)
	}
	if err := call.Store(&fd); err != nil {
		sc.Close()
		return nil, fmt.Errorf("OpenPipeWireRemote fd: %w", err)
	}
	sc.fd = int(fd)
	debugf("screencast: session ready node=%d fd=%d", sc.nodeID, sc.fd)
	return sc, nil
}

// request invokes a portal method that returns a Request handle and waits for its
// Response, matching on the predicted request path (sender + token).
func (sc *screenCast) request(ctx context.Context, label string, do func(tok string) *dbus.Call) (map[string]dbus.Variant, error) {
	tok := fmt.Sprintf("pgev_%s_%d", label, atomic.AddUint64(&scSeq, 1))
	want := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sc.sender + "/" + tok)
	call := do(tok)
	if call.Err != nil {
		return nil, fmt.Errorf("%s call: %w", label, call.Err)
	}
	var handle dbus.ObjectPath
	_ = call.Store(&handle)
	debugf("screencast: %s want=%s handle=%s", label, want, handle)
	for {
		select {
		case sig := <-sc.sigCh:
			if (sig.Path != want && sig.Path != handle) || len(sig.Body) < 2 {
				continue
			}
			code, _ := sig.Body[0].(uint32)
			if code != 0 {
				return nil, fmt.Errorf("%s not granted (response %d)", label, code)
			}
			results, _ := sig.Body[1].(map[string]dbus.Variant)
			return results, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("%s timed out/cancelled: %w", label, ctx.Err())
		}
	}
}

// Close ends the session (closing the bus connection) and the PipeWire fd if it
// hasn't been handed off to the consumer process.
func (sc *screenCast) Close() {
	if sc == nil {
		return
	}
	if sc.fd >= 0 {
		syscall.Close(sc.fd)
		sc.fd = -1
	}
	if sc.conn != nil {
		sc.conn.Close()
	}
}

// firstStreamNode extracts the first PipeWire node id from a Start response's
// "streams" field (array of (uint32 node, a{sv} props)).
func firstStreamNode(res map[string]dbus.Variant) (uint32, error) {
	v, ok := res["streams"]
	if !ok {
		return 0, fmt.Errorf("screencast: no streams in response")
	}
	arr, ok := v.Value().([]interface{})
	if !ok || len(arr) == 0 {
		return 0, fmt.Errorf("screencast: empty streams")
	}
	first, ok := arr[0].([]interface{})
	if !ok || len(first) < 1 {
		return 0, fmt.Errorf("screencast: malformed stream")
	}
	node, ok := first[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("screencast: stream node id not a uint32")
	}
	return node, nil
}

func sanitizeBusName(name string) string {
	// ":1.234" -> "1_234"
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == ':':
			continue
		case c == '.':
			out = append(out, '_')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// startRecordingPortal records the Wayland screen via the ScreenCast portal +
// PipeWire, encoded by a GStreamer pipeline (ffmpeg can't read PipeWire). The
// PipeWire fd is handed to gst-launch via ExtraFiles (fd 3 in the child).
func startRecordingPortal(outPath string) (*Recorder, error) {
	gst, err := exec.LookPath("gst-launch-1.0")
	if err != nil {
		return nil, fmt.Errorf("gst-launch-1.0 not found — install gstreamer to record on Wayland")
	}
	enc, encArgs := pickH264Enc()
	if enc == "" {
		return nil, fmt.Errorf("no GStreamer H.264 encoder found — install e.g. gstreamer1-plugin-openh264")
	}

	// Bound the session setup with a watchdog so a stuck bus/portal can't hang.
	ctx, cancel := context.WithTimeout(context.Background(), scSetupTimeout)
	defer cancel()
	type result struct {
		sc  *screenCast
		err error
	}
	ch := make(chan result, 1)
	go func() { sc, err := startScreenCast(ctx); ch <- result{sc, err} }()
	var sc *screenCast
	select {
	case r := <-ch:
		sc, err = r.sc, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("screencast setup timed out: %w", ctx.Err())
	}
	if err != nil {
		return nil, err
	}

	// Hand the PipeWire fd to gst-launch as fd 3 (first ExtraFiles entry).
	pwFile := os.NewFile(uintptr(sc.fd), "pipewire")
	sc.fd = -1 // ownership now belongs to pwFile

	pipeline := []string{"-e",
		"pipewiresrc", "fd=3", fmt.Sprintf("path=%d", sc.nodeID), "do-timestamp=true",
		"!", "videoconvert",
		"!", enc,
	}
	pipeline = append(pipeline, encArgs...)
	pipeline = append(pipeline, "!", "h264parse", "!", "mp4mux", "!", "filesink", "location="+outPath)

	cmd := exec.Command(gst, pipeline...)
	cmd.ExtraFiles = []*os.File{pwFile}
	if os.Getenv("PGEVIDENCE_DEBUG") != "" {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &bytes.Buffer{}
	}
	if err := cmd.Start(); err != nil {
		pwFile.Close()
		sc.Close()
		return nil, fmt.Errorf("gst-launch start: %w", err)
	}
	pwFile.Close() // child holds its own dup
	debugf("screencast: recording node=%d enc=%s -> %s", sc.nodeID, enc, outPath)
	return &Recorder{cmd: cmd, cleanup: sc.Close}, nil
}

// pickH264Enc returns the first available GStreamer H.264 encoder element and any
// extra properties. Software encoders are preferred for pipeline simplicity.
func pickH264Enc() (string, []string) {
	type enc struct {
		name string
		args []string
	}
	for _, e := range []enc{
		{"x264enc", []string{"tune=zerolatency", "speed-preset=veryfast", "bitrate=6000"}},
		{"openh264enc", nil},
		{"vah264enc", nil},
		{"vaapih264enc", nil},
	} {
		if gstHasElement(e.name) {
			return e.name, e.args
		}
	}
	return "", nil
}

func gstHasElement(name string) bool {
	insp, err := exec.LookPath("gst-inspect-1.0")
	if err != nil {
		return false
	}
	return exec.Command(insp, name).Run() == nil
}
