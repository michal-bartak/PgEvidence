//go:build linux

package capture

import (
	"context"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

// portalTimeout bounds the whole portal exchange. Long enough to accept a
// permission dialog, short enough that a stuck/ignored prompt can't hang the run.
const portalTimeout = 30 * time.Second

var portalSeq uint64

// screenshotPortal captures the full screen via the xdg-desktop-portal Screenshot
// interface (org.freedesktop.portal.Screenshot). This is the supported way to
// capture on Wayland (GNOME/KDE/wlroots) and is correct at any display scaling,
// unlike the X11/XWayland path which clips under fractional scaling. The portal
// writes a PNG and returns its URI; we copy it to outPath. GNOME may prompt for
// permission — that's inherent to the Wayland security model. The portal always
// captures the whole desktop, so the result is cropped to the selected display
// (displayIndex) before it is written to outPath.
func screenshotPortal(ctx context.Context, displayIndex int, outPath string) error {
	// Bound the whole exchange and honour the caller's cancellation (run Cancel).
	ctx, cancel := context.WithTimeout(ctx, portalTimeout)
	defer cancel()

	// Watchdog: the D-Bus connection setup (SessionBusPrivate/Auth/Hello/AddMatch)
	// isn't context-aware and can block in a broken session env (e.g. launched from
	// an IDE's sandboxed integrated terminal). Run the whole exchange in a goroutine
	// and return on ctx timeout/cancel so a stuck bus can never hang the run. A
	// blocked goroutine may linger, but only in such pathological envs (one per
	// capture) — far better than stalling.
	done := make(chan error, 1)
	go func() { done <- portalCapture(ctx, displayIndex, outPath) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("portal screenshot timed out/cancelled after up to %s: %w", portalTimeout, ctx.Err())
	}
}

func portalCapture(ctx context.Context, displayIndex int, outPath string) error {
	debugf("portal: connecting to session bus")
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return fmt.Errorf("dbus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		return fmt.Errorf("dbus hello: %w", err)
	}
	debugf("portal: connected, bus name=%q", conn.Names()[0])

	token := fmt.Sprintf("pgevidence_%d_%d", os.Getpid(), atomic.AddUint64(&portalSeq, 1))

	// The portal emits Response on a path derived from OUR bus name + token, which
	// often differs from the handle the method returns. Compute that predicted path
	// and match on it (the spec-recommended way) — matching the returned handle is
	// unreliable and was causing us to ignore the reply and stall until timeout.
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	wantPath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)

	// Subscribe before calling, so we can't miss the reply.
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(wantPath),
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return fmt.Errorf("dbus match: %w", err)
	}
	sigCh := make(chan *dbus.Signal, 4)
	conn.Signal(sigCh)

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	opts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"interactive":  dbus.MakeVariant(false),
		"modal":        dbus.MakeVariant(false),
	}
	var handle dbus.ObjectPath
	debugf("portal: calling Screenshot, wantPath=%s", wantPath)
	// CallWithContext so a hung/unanswered portal method can't block forever.
	call := obj.CallWithContext(ctx, "org.freedesktop.portal.Screenshot.Screenshot", 0, "", opts)
	if call.Err != nil {
		return fmt.Errorf("portal Screenshot call: %w", call.Err)
	}
	_ = call.Store(&handle) // handle is informational; we match on wantPath
	debugf("portal: call returned handle=%s; waiting for Response", handle)

	for {
		select {
		case sig := <-sigCh:
			debugf("portal: signal path=%s members=%d", sig.Path, len(sig.Body))
			// Accept the Response on our predicted request path (or the returned
			// handle, if the portal happens to use it).
			if (sig.Path != wantPath && sig.Path != handle) || len(sig.Body) < 2 {
				continue
			}
			code, _ := sig.Body[0].(uint32)
			if code != 0 {
				// 1 = cancelled by user, 2 = ended some other way.
				return fmt.Errorf("portal screenshot was not granted (response %d)", code)
			}
			results, _ := sig.Body[1].(map[string]dbus.Variant)
			uriVar, ok := results["uri"]
			if !ok {
				return fmt.Errorf("portal returned no uri")
			}
			uri, _ := uriVar.Value().(string)
			debugf("portal: got uri=%s", uri)
			return savePortalResult(uri, displayIndex, outPath)
		case <-ctx.Done():
			debugf("portal: ctx done (timeout/cancel)")
			return fmt.Errorf("portal screenshot timed out/cancelled after up to %s: %w", portalTimeout, ctx.Err())
		}
	}
}

// savePortalResult reads the portal's file:// result PNG (the whole desktop),
// crops it to the selected display, writes it to outPath, and removes the
// portal's temp copy.
func savePortalResult(uri string, displayIndex int, outPath string) error {
	u, err := url.Parse(uri)
	if err != nil || u.Path == "" {
		return fmt.Errorf("bad portal uri %q: %v", uri, err)
	}
	src := u.Path
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open portal result: %w", err)
	}
	img, derr := png.Decode(in)
	in.Close()
	if derr != nil {
		return fmt.Errorf("decode portal result: %w", derr)
	}
	img = cropToSelectedDisplay(img, displayIndex)

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	if err := png.Encode(out, img); err != nil {
		out.Close()
		os.Remove(outPath)
		return fmt.Errorf("encode screenshot: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(outPath)
		return err
	}
	// Best-effort cleanup of the portal's temp file (often under ~/.cache or /tmp).
	if strings.Contains(src, "/tmp/") || strings.Contains(src, "/.cache/") {
		os.Remove(src)
	}
	return nil
}
