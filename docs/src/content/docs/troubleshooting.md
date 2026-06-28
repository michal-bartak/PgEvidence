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

## Linux: screenshot is clipped, or "no screenshot tool succeeded"

On Wayland (e.g. GNOME with fractional/HiDPI scaling) the built-in X11 capture grabs only
part of the screen, so PgEvidence uses a desktop screenshot tool instead. It tries
`gnome-screenshot`, then `spectacle` (KDE), then `grim` (wlroots). The `.deb`/`.rpm`
packages depend on `gnome-screenshot`; if you installed another way or use a different
desktop, install one of those tools:

```bash
sudo dnf install gnome-screenshot     # Fedora / GNOME
sudo apt install gnome-screenshot     # Debian / Ubuntu
```

On a genuine **X11** session the built-in capture is used and no extra tool is needed.

## Linux: recorded video is black (Wayland)

Screen **recording** (the optional MP4) currently uses `x11grab`, which produces a black
video under Wayland (only the cursor shows). Use the **screenshots** (the primary evidence)
instead, or run the app in an **X11/Xorg** session for video. Native Wayland recording is a
known limitation.
