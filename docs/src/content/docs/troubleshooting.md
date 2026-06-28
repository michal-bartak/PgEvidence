---
title: Troubleshooting
description: Solutions for common PgEvidence issues
---

## macOS keeps prompting "PgEvidence is recording your screen"

That dialog is macOS's built-in screen-recording **reminder** (Sequoia and later), which is
**separate** from the Screen Recording toggle in System Settings. macOS shows it on first use
after an app is installed or updated, then periodically — **even for apps that already have
permission**.

It only affects **video recording** (a continuous capture stream); **screenshots** use Apple's
`screencapture` tool and don't trigger it. Because capture is already permitted, recording
starts immediately — so the reminder can appear in the first seconds of the video. Click
**Allow**; it won't return until the next update or month.

:::note
PgEvidence does **not** record audio — ffmpeg captures video only. The "and audio" wording is
macOS's generic text.
:::

## Screenshots come out blank (macOS)

Grant **Screen Recording** to PgEvidence (System Settings → Privacy & Security → Screen
Recording), then **quit and reopen** the app — a permission grant doesn't apply to an
already-running process.

## macOS won't open the app ("unidentified developer")

The app is published as unsigned. If downloaded with use of browser or other program that cooperates in Apple's Gatekeeper program, such a file is marked for quarantine. to avoid that
* either download the DMG using `curl -LJO <url>`command (no quarantine flag is set), or
* after installation run once:
    ```bash
    xattr -d com.apple.quarantine /Applications/pgevidence.app
    ```

## "psql not found"

Install the PostgreSQL client. PgEvidence auto-detects it on `PATH` and in common locations
(Homebrew, Postgres.app, EDB, `Program Files`). If it's elsewhere, set it in
**Settings → Environment → psql path** (Browse…). Linux packages install the client
automatically.

## Windows SmartScreen warning

The installer is unsigned. Choose **More info → Run anyway**.

## Linux: sluggish UI or window on the wrong monitor

The provided packages launch the app under XWayland (`GDK_BACKEND=x11`) automatically. If you
run the binary directly, prefix it:

```bash
GDK_BACKEND=x11 pgevidence
```

## Linux: screenshots on Wayland

On Wayland (e.g. GNOME, the Fedora default) the built-in X11 capture clips under
fractional/HiDPI scaling, so PgEvidence captures through the **desktop portal**
(`xdg-desktop-portal`) — the supported method, which works at any scaling and includes the
top-bar clock. Depending on your desktop, the portal may show a one-time permission prompt.

The `.deb`/`.rpm` packages depend on `xdg-desktop-portal`; the GNOME/KDE portal backend is
part of the desktop. If capture fails, ensure the portal is installed and running:

```bash
sudo dnf install xdg-desktop-portal xdg-desktop-portal-gnome   # Fedora / GNOME
```

On a genuine **X11/Xorg** session the built-in capture is used instead — full screen, no
dialog. (Note: `gnome-screenshot` is **not** used; it's broken on recent GNOME.)

**Launch from the app icon (or a normal terminal).** Running PgEvidence from an IDE's
integrated terminal (VS Code, GitKraken) can stall capture, because those apps pass a
sandboxed/modified session environment to the portal. Launched normally it works; capture
now also times out instead of hanging if the session bus is unreachable.

**The screen flashes on each capture.** That's GNOME's own screenshot flash; it happens
after the image is taken, so it is **not** in the saved PNG. To suppress it, disable
animations globally: `gsettings set org.gnome.desktop.interface enable-animations false`.

## Linux: video recording on Wayland

On Wayland, screen **recording** (the optional MP4) goes through the **ScreenCast portal +
PipeWire**, encoded by **GStreamer** (ffmpeg can't capture Wayland or read PipeWire). When a
run with video starts, GNOME shows a **"share your screen" picker once** — choose your
monitor and click Share. On X11/Xorg and macOS/Windows, ffmpeg is used as before.

It needs `gst-launch-1.0`, a PipeWire source plugin, and an H.264 encoder. If recording
can't start, the run continues with screenshots and logs why. Install the pieces:

```bash
# Fedora / GNOME
sudo dnf install gstreamer1 gstreamer1-plugins-base gstreamer1-plugins-good \
  pipewire-gstreamer gstreamer1-plugin-openh264
# Debian / Ubuntu
sudo apt install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good \
  gstreamer1.0-pipewire gstreamer1.0-plugins-ugly
```

The encoder is auto-detected (`x264enc`, `openh264enc`, or VAAPI `vah264enc`/`vaapih264enc`).
