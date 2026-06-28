//go:build linux

package capture

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

// portalTimeout bounds the whole portal exchange. It is generous because GNOME
// may show a permission dialog the user has to accept before the capture runs.
const portalTimeout = 60 * time.Second

var portalSeq uint64

// screenshotPortal captures the full screen via the xdg-desktop-portal Screenshot
// interface (org.freedesktop.portal.Screenshot). This is the supported way to
// capture on Wayland (GNOME/KDE/wlroots) and is correct at any display scaling,
// unlike the X11/XWayland path which clips under fractional scaling. The portal
// writes a PNG and returns its URI; we copy it to outPath. GNOME may prompt for
// permission — that's inherent to the Wayland security model.
func screenshotPortal(outPath string) error {
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

	// Subscribe to Request.Response before calling, so we can't miss the reply.
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return fmt.Errorf("dbus match: %w", err)
	}
	sigCh := make(chan *dbus.Signal, 4)
	conn.Signal(sigCh)

	token := fmt.Sprintf("pgevidence_%d_%d", os.Getpid(), atomic.AddUint64(&portalSeq, 1))
	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	opts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"interactive":  dbus.MakeVariant(false),
		"modal":        dbus.MakeVariant(false),
	}
	var handle dbus.ObjectPath
	call := obj.Call("org.freedesktop.portal.Screenshot.Screenshot", 0, "", opts)
	if call.Err != nil {
		return fmt.Errorf("portal Screenshot call: %w", call.Err)
	}
	if err := call.Store(&handle); err != nil {
		return fmt.Errorf("portal handle: %w", err)
	}

	deadline := time.After(portalTimeout)
	for {
		select {
		case sig := <-sigCh:
			if sig.Path != handle || len(sig.Body) < 2 {
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
			return copyPortalResult(uri, outPath)
		case <-deadline:
			return fmt.Errorf("portal screenshot timed out after %s", portalTimeout)
		}
	}
}

// copyPortalResult copies the portal's file:// result PNG to outPath and removes
// the portal's temp copy.
func copyPortalResult(uri, outPath string) error {
	u, err := url.Parse(uri)
	if err != nil || u.Path == "" {
		return fmt.Errorf("bad portal uri %q: %v", uri, err)
	}
	src := u.Path
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open portal result: %w", err)
	}
	defer in.Close()
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(outPath)
		return fmt.Errorf("copy portal result: %w", err)
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
