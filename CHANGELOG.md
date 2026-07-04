# Changelog

All notable changes to **PgEvidence** are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.1] - 2026-07-04

### Added
- An **About** dialog (opened from the ⓘ button in the header) 

## [1.2.0] - 2026-07-03

### Added
- Copy the config directory path to the clipboard from Settings → Environment.

### Changed
- Removed the monitor selector; capture always follows the app-window monitor.
- Removed the Displays count from Settings → Environment.

### Fixed
- [Windows] Video recorded all monitors instead of the recognized one.
- Query saves are now atomic (temp file + rename), so a crash or forced quit
  mid-save can no longer truncate or empty `queries.json`.

## [1.1.0] - 2026-07-03

### Added
- Selecting source screen for screenshots automatically. Removed screen selector
- The per-query `.sql` file is now always written and gets its own SHA-256 checksum

### Changed
- Removed the optional "Save each query as a .sql file" setting
- The Run-page ZIP control now cycles through four states on click (off → on/no
  password → on/random password → on/password)

### Fixed
- [Linux] Fixed screenshots that were grabbed from all monitors.

## [1.0.0] - 2026-06-29

### Added
- Linux Wayland screenshots via `xdg-desktop-portal` (scaling-correct, captures the real
  screen incl. the OS clock); multi-monitor captures are cropped to the selected display.
- Linux Wayland video recording via the ScreenCast portal + GStreamer (PipeWire source,
  runtime-detected H.264 encoder).
- macOS Screen Recording flow: separate **Grant permission** and **Open Settings** buttons;
  the app registers itself in the Screen Recording list via a real `screencapture` attempt.
- **Auto** monitor selection: capture the display showing the app window (resolved natively
  on macOS via the CoreGraphics window list).
- Window size persists across launches.
- Read-only / read-write indicator chip.
- GitHub Pages documentation site (Astro Starlight) + README troubleshooting.

### Changed
- Run-page controls (screenshots/video/ZIP/connection) are now per-run and no longer persist;
  Settings remains the persisted source of truth.
- Themed native controls on Linux: dropdowns, number steppers, checkboxes/radios, scrollbar.
- System-facing name shown as cased **PgEvidence**; build artifacts stay lowercase.
- Windows: hide the `psql`/`ffmpeg` child console window.

### Fixed
- Invisible Grant-permission button in light theme.
- Hint popover clipped at window edges (now clamped to the viewport).
- Scrollbar color now follows the active theme.
- Hardened from a code review; cancelled runs are no longer archived.

## [0.2.0] - 2026-06-26

### Added
- Themes (system / light / dark), Run-tab controls, help hints, and Settings auto-save.
- "Exclude video from ZIP" archive option.

### Changed
- Renamed the app to **PgEvidence**.

## [0.1.0] - 2026-06-25

### Added
- Initial release: cross-platform desktop app (Go + Wails v2, Svelte-TS frontend).
- Runs a maintained set of SQL queries against PostgreSQL via the system `psql --csv`.
- Tamper-evident evidence per query: CSV result, SHA-256 checksum sidecar, and a
  full-desktop screenshot capturing the OS clock as proof-of-time.
- Run `manifest.json` with a checksum sidecar; read-only enforcement; no persisted DB password.
- Optional per-query `.sql` file and prune-after-zip.
- ZIP archiving of run output (ZipCrypto; none / explicit / auto password modes).
- Query import from a JSON set or a SQL script (split on top-level `;`, query name taken
  from the preceding comment).
- Cross-platform release packaging: macOS DMG, Windows MSI, Linux deb + rpm.
