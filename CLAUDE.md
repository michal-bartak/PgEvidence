# CLAUDE.md — PgEvidence

Developer/agent notes for this repo. Read this before changing code. **This file
is the maintained in-repo design record — keep the Decision log (bottom) and these
notes current when decisions change.**

## What this is

A cross-platform desktop app (Go + Wails v2, Svelte-TS frontend) that runs a
maintained set of SQL queries against PostgreSQL one-by-one and produces
**tamper-evident audit evidence** for each query:

- a CSV result file (`NNNN_<slug>.csv`) via the system `psql --csv`,
- optionally the query text itself (`NNNN_<slug>.sql`, no checksum; `SaveQuerySQL`),
- a sha256sum-compatible checksum sidecar (`NNNN_<slug>.csv.sha256`),
- a **full-desktop screenshot** (`NNNN_<slug>.png`) taken after a configurable
  dwell time, so the OS clock is captured in-frame as proof-of-time,
- a run `manifest.json` plus a sidecar `manifest.json.sha256` over the manifest.

The purpose: auditors periodically request ad-hoc extracts; this tool makes the
runs reproducible and self-evidencing. Files for one run live in
`<outputDir>/audit-run-YYYYMMDD-HHMMSS/`.

## Locked design decisions (do not silently reverse)

- **Evidence = full-screen screenshots (single monitor), primary.** Must include
  the OS clock, so we capture the real display — not the app's own rendered view.
  Video (MP4 via ffmpeg) is optional, off by default, auto-skipped if ffmpeg absent.
- **No password is ever persisted.** Config stores only host/port/db/user/sslmode.
  Password comes from `~/.pgpass` or an in-memory session prompt, passed to `psql`
  only via `PGPASSWORD` on the child process env.
- **Read-only enforcement** is on by default, implemented by setting
  `PGOPTIONS=-c default_transaction_read_only=on` on the psql child env (NOT via an
  extra `-c` statement — that would risk contaminating CSV output).
- **Multi-database is future-ready, not built.** Connections are modelled as a
  list from day one; a run targets one selected connection. Running one set across
  many connections is a deliberate later extension.
- Checksums are SHA-256. Connection definitions carry no secrets.
- **Evidence file naming is uniform:** `name.ext` → `name.ext.sha256` for every
  checksummed file, including `manifest.json.sha256` (no special `run.sha256`).
- **Version has one source:** the repo-root `VERSION` file, embedded into `main.go`
  (`//go:embed VERSION` → `AppVersion`), recorded in the manifest and shown in the UI.
- **Cased display name vs lowercase artifacts.** Human-facing places use
  "PgEvidence": `wails.json` `productName` (→ `CFBundleName` + `CFBundleDisplayName`,
  so Finder/Applications/Screen-Recording show it), the config dir
  (`os.UserConfigDir()/PgEvidence`, with case-fix migration), and the default output
  folder. Lowercase `pgevidence` stays for build artifacts: `outputfilename` (binary,
  `.dmg`/`.msi`/`.deb`/`.rpm` names), the Go module path, and the bundle id
  `com.wails.pgevidence`.
- **App icon has one source drawing:** `scripts/gen_icon.py` (mirrors
  `build/appicon.svg`) emits `build/appicon.png`, a complete `build/appicon.icns`,
  and `build/windows/icon.ico`. See the icon gotcha below — Wails' own icon
  generation is insufficient on macOS and absent for Windows.

## Architecture / package map

Go backend drives the run loop and emits Wails runtime events; the Svelte frontend
is a thin live view. `App` (in `app.go`) is the Wails-bound object and implements
`runner.UI`.

```
main.go                      Wails bootstrap (window 1200x820, bindings)
app.go                       App struct: bound methods + runner.UI (Emit/BringToFront)
internal/config/             Config + Connection list; JSON in os.UserConfigDir()/PgEvidence
internal/store/              Query CRUD: Upsert/Delete/Move/ReplaceAll/Import/Export
                             Import parses JSON set OR splits a SQL script on top-level ';'
                             (comment-aware); query name = the free text/comment
                             before it, split at the first SQL keyword (nameAndSQL),
                             excluded from the stored SQL; see store_test.go
internal/psql/               Runs system psql (--csv, --no-psqlrc, ON_ERROR_STOP=1) to a file
internal/checksum/           SHA-256 + WriteSidecar (coreutils "<hex>  <name>" format)
internal/capture/            capture.go: full-display screenshot (kbinani/screenshot)
                             recorder.go: optional ffmpeg recorder (experimental, per-OS input)
internal/manifest/           Manifest struct + Write (manifest.json + manifest.json.sha256)
internal/archive/            ZIP packaging (yeka/zip, ZipCrypto); Create/GeneratePassword/WritePwdFile
internal/proc/               proc.Hide(*exec.Cmd): Windows-only CREATE_NO_WINDOW so psql/ffmpeg
                             children don't flash a console; no-op elsewhere (build-tagged)
internal/runner/             Orchestrates the run: loop, run:* events, dwell, screenshot, manifest
frontend/src/App.svelte      Shell: tabs (Queries/Run/Settings), loads env+config+queries on mount;
                             seeds runOpts from config; Settings-tab "Saved" flash; window-size persist
frontend/src/stores.ts       Svelte stores (cfg, queries, env, activeTab, runOpts, savedTick) + payloads
frontend/src/theme.ts        applyTheme(system|light|dark): sets <html data-theme>; Linux uses IsSystemDark
frontend/src/views/          Queries.svelte, Run.svelte, Settings.svelte
frontend/src/components/      Hint.svelte (fixed-positioned "?" hover popover)
frontend/wailsjs/            GENERATED bindings — never hand-edit
```

### Run loop (internal/runner/runner.go, started by App.StartRun on a goroutine)

Per enabled query, in order: emit `run:query` → `psql.RunToFile` → checksum sidecar
→ `previewAndCount` (single CSV pass: header + top-N rows + exact row count) → emit
`run:result` → settle ~350ms for paint → full-screen screenshot → emit `run:dwell`
(the UI countdown starts here, so it matches the real remaining time) → hold
`dwellSeconds` (cancellable). Then write manifest + emit `run:done`. Cancellation
flows through a `context.Context` cancelled by `App.CancelRun`.

### Frontend ↔ backend contract

- Bound methods are typed in `frontend/wailsjs/go/main/App.d.ts`; models in
  `frontend/wailsjs/go/models.ts`. Regenerate with `wails generate module` after
  changing any bound Go signature or struct.
- Events are untyped over the bridge; their payload shapes are declared in
  `frontend/src/stores.ts` (`QueryPayload`, `ResultPayload`, `DonePayload`) and must
  be kept in sync with the `map[string]interface{}` payloads emitted in `runner.go`.
  Event names: `run:start`, `run:query`, `run:result`, `run:dwell`, `run:log`, `run:done`.
- Queries reorder by **drag-and-drop** (native HTML5 DnD in `Queries.svelte`),
  persisted via `ReplaceAllQueries` (which renumbers `order` by index).
- Starting a run first calls `SaveConfig` with the on-screen config, so a run always
  uses what's displayed (avoids the "unsaved Settings edit" divergence).
- **Archiving is frontend-orchestrated after `run:done`** (not in the runner), so the
  explicit-password prompt can happen in the UI. `Run.svelte` calls `ArchiveRun`
  (none/explicit) or `ArchiveRunAuto`; the zip is written **inside** the run folder.
  When `DeleteSourcesAfterZip` is set, the UI then calls `PruneRunDir` (which
  refuses to delete unless the archive exists and is non-empty), leaving only the zip.

## Build / dev commands

```
wails dev                 # live dev (Vite HMR + Go)
wails build               # production .app/.exe bundle -> build/bin/
wails generate module     # regenerate frontend/wailsjs bindings
go build ./... && go vet ./...
cd frontend && npm install && npm run build

make icon                 # regenerate build/appicon.png from scripts/gen_icon.py
make cert                 # one-time: create self-signed code-signing cert (macOS)
make dist                 # wails build + re-sign with stable identity (macOS)
make reset-screen-perm    # tccutil reset ScreenCapture (clear stale TCC grant)
```

Bump the app version by editing the repo-root `VERSION` file (embedded at build).

## Release / packaging

`.github/workflows/release.yml` (manual `workflow_dispatch`) builds installers for
all three platforms on a `vX.Y.Z` tag (3-OS matrix), modelled on the `osc` project:
- **macOS:** `darwin/universal` → drag-to-Applications **DMG** (Pillow background +
  Finder AppleScript layout). The committed `build/appicon.icns` is copied into the
  bundle, then ad-hoc re-signed (`codesign --force --sign -`, **no `--deep`**).
- **Windows:** **WiX MSI** (`build/windows/installer/product.wxs`, fresh UpgradeCode);
  banner/dialog BMPs generated from `build/appicon.png` via System.Drawing.
- **Linux:** **deb + rpm** via `fpm`; runtime deps `libgtk-3-0`/`gtk3`,
  `libwebkit2gtk-4.1-0`/`webkit2gtk4.1`, plus **`postgresql-client`/`postgresql`**
  (hard psql dep) and `xdg-utils`. The `.desktop` runs with `GDK_BACKEND=x11` so
  WebKit and the X11 screenshot path work under Wayland/XWayland.

Gotchas: the tag must equal `v$(cat VERSION)` (the `prepare` job enforces it) and
`wails.json` `info.productVersion` is jq-synced from `VERSION` at build. Linux build
needs `build:tags: webkit2_41` (in `wails.json`) + `libwebkit2gtk-4.1-dev`. Ships
unsigned (Gatekeeper/SmartScreen documented in the release notes).

**macOS signing (why `make dist`, not `wails build`):** `wails build` ad-hoc
signs, and an ad-hoc signature's hash changes every build, so the Screen
Recording (TCC) grant does NOT persist across rebuilds. Signing with a stable
self-signed identity ("PgEvidence Dev", created by
`scripts/create-signing-cert.sh`) makes the grant stick. `make dist` builds then
`codesign --force --deep --sign "PgEvidence Dev"`. After the FIRST signed
build, run `make reset-screen-perm` and grant once; subsequent signed builds keep
it. (`security find-identity -v` shows 0 valid identities — expected, the cert is
untrusted; codesign still signs and TCC keys on the signing identity, not trust.)

Module path is `pgevidence`; internal packages import as `pgevidence/internal/...`.

## Platform gotchas

- **macOS screenshots use `/usr/sbin/screencapture`**, NOT kbinani. The kbinani
  CoreGraphics path (`CGDisplayCreateImage`, deprecated) composites the app menu
  but DROPS the right-side menu-bar extras (clock + status icons, drawn by
  ControlCenter) on recent macOS — which defeats the proof-of-time requirement.
  `screencapture -x -t png -D <n>` captures the full menu bar including the clock.
  Display numbers there are 1-based, so `MonitorIndex` (0-based) maps to index+1.
  (Quirk: `screencapture -R` region capture rejects `y=0`; full `-D` capture is fine.)
- **macOS Screen Recording permission** is still required; when denied,
  `screencapture` exits 1 with "could not create image from display". The app
  detects access via `capture.HasScreenAccess` (CGPreflightScreenCaptureAccess,
  cgo, `access_darwin.go`) and can surface the system prompt via
  `RequestScreenAccess` (CGRequestScreenCaptureAccess). `EnvInfo.screenAccess`
  drives a warning banner + "Grant permission" button in `App.svelte`. NOTE: TCC
  grants don't apply to an already-running process — the app must be **quit and
  reopened** after enabling. Non-darwin stubs in `access_other.go` return true.
- Linux/Windows use the kbinani path (X11/GDI); `isBlank` guards all-black captures.
  Linux Wayland may need a portal.
- **PATH for GUI-launched apps:** a Finder/Launchpad-launched `.app` inherits only
  a minimal PATH (`/usr/bin:/bin:/usr/sbin:/sbin`), so `exec.LookPath("psql")` fails
  even though `wails dev` (terminal-launched) finds it. `psql.binary()` therefore
  falls back to well-known install dirs (Homebrew, MacPorts, Postgres.app, EDB,
  Windows `Program Files`) via `candidatePaths()`; `capture.FFmpegPath()` does the
  same. All psql calls go through the resolved absolute path.
- `psql` must be installed (PATH or a common location). `ffmpeg` only if video is enabled.
- Single-monitor capture by default (`config.MonitorIndex`, default 0).
- **Windows: hide the child console window.** Launching a console program
  (`psql.exe`, `ffmpeg.exe`) from a GUI app flashes a shell window on each call.
  Every console child runs through `proc.Hide(cmd)` first — Windows-only, it sets
  `SysProcAttr.HideWindow` + `CREATE_NO_WINDOW` (`0x08000000`); a `//go:build
  !windows` stub is a no-op. Wired into `psql.RunToFile/Test/Detect` and the
  ffmpeg recorder. (Not macOS `screencapture`/`open` or Windows `explorer` — GUI,
  no console.) Can't be reproduced on macOS; verify with `GOOS=windows go build`.
- **Window size persists across launches; save the OS size, not the viewport.**
  `config.WindowWidth/Height` (omitempty) are restored in `main.go` (floored at
  `minWinW/H` 900×600, default 1200×820). The frontend saves on a debounced
  `resize` via Wails `WindowGetSize()` (the OS window size) through
  `App.SaveWindowSize`. Using `window.innerWidth/innerHeight` (the webview
  viewport, which excludes the OS chrome) makes the window **shrink a little every
  launch** — the bug the `osc` project hit (its commit "Fix to viewport size on
  Windows"). MinWidth/MinHeight also block dragging below the floor.
- **macOS "Grant permission" does exactly ONE thing, never both.**
  `CGRequestScreenCaptureAccess` only ever shows the prompt once; once the app is
  listed (even if disabled) re-requesting is a silent no-op. The system prompt
  *itself* has an "Open System Settings" button, so showing the prompt AND opening
  Settings is redundant/confusing. `App.GrantScreenAccess` (darwin) **always** runs
  `capture.RegisterForScreenAccess` first (a real capture attempt — the only
  reliable way to get listed; self-regulating: it shows the prompt only when status
  is "not determined", silent when denied), then branches on the persisted
  `config.ScreenAccessPrompted` flag: **first time** → just set the flag (the
  prompt's own button is enough); **afterwards** → `OpenScreenRecordingSettings`
  (`open x-apple.systempreferences:...?Privacy_ScreenCapture`). The always-register
  step repairs the state where a prior prompt left the app *unlisted* — there is NO
  public API to query list membership (CGPreflight can't tell "not listed" from
  "listed but disabled"), so we guarantee registration rather than detect it. The
  flag is needed because the TCC API can't distinguish "not yet asked" from "asked
  and denied". The banner also has a session-only ✕ dismiss (non-persisted;
  reappears next launch if access is still missing).
- **Registering the app in the Screen Recording list needs a real capture, not
  `CGRequest`.** Because real screenshots go through the external `screencapture`
  tool, the app process never calls an in-process capture API, so on a fresh
  install `CGRequestScreenCaptureAccess` alone often fails to add the app to the
  list (user has to add it by hand). `RequestScreenAccess` therefore also fires a
  throwaway, silent `screencapture` (`registerViaScreencapture` in `capture.go`):
  a denied attempt registers the app via the responsible-process chain — the same
  way a normal run does. (Can't use `CGDisplayCreateImage` to trigger it: hard-
  unavailable in the macOS 15 SDK.) NOTE for testing with `make reset-screen-perm`:
  that resets TCC but not the config flag, so also clear `screenAccessPrompted`
  (or delete config.json) to re-test the first-time prompt path.
- **App icon (cross-project gotcha, confirmed in the `osc` project too):**
  - macOS: the `.icns` Wails generates **omits the `@1x` sizes**, so Finder/Dock/
    cmd-tab fall back to a generic icon. Fix: generate a complete `.icns`
    (`make icon`) and copy it over `Contents/Resources/iconfile.icns`, then re-sign
    — `make dist` does this via the `fix-icon` step (and re-signs *after* the copy,
    or the signature wouldn't cover the new icns).
  - Windows: Wails **never** regenerates `build/windows/icon.ico` from
    `appicon.png`; the `.exe` icon comes from that `.ico`. `make icon` rebuilds it.
  - macOS caches icons hard: after a rebuild run `lsregister -f <app>` + `touch`
    (both in `make dist`); a running app must be **quit and reopened** for cmd-tab/
    Dock to pick up the new icon.

## Verification

`go build ./...`, `go vet ./...`, `npm run build`, and `wails build` all pass.

The psql→CSV→checksum→preview→manifest path was verified end-to-end against a
live PostgreSQL 17.7 instance (via a temporary `cmd/verify` harness, since removed,
that drove `runner.Run` with a no-op UI and `Screenshots:false`). Confirmed:
- multi-row CSV output and correct header/row/rowCount preview parsing,
- `sha256sum -c *.csv.sha256` → OK for every successful query,
- `manifest.json.sha256` verifies against `manifest.json`,
- read-only enforcement blocks writes (`CREATE TABLE` → "cannot execute … in a
  read-only transaction", recorded as status `error`, nothing created in the DB),
- manifest contains no password and records psql version + connection provenance.

**Still requires manual GUI check:** screenshots (`NNNN_*.png`) — needs a real
display and, on macOS, Screen Recording permission for the running app. Verify by
launching via `wails dev`, running a set, and confirming each PNG shows the app
plus the OS clock. To re-verify the data pipeline later, recreate a small
`cmd/verify/main.go` like the one described above.

## Status / TODO

- Core backend + frontend implemented; `wails build` packages successfully.
- Data pipeline verified end-to-end against live PostgreSQL 17.7 (see Verification).
- Screenshot capture still needs a manual GUI check (display + macOS permission).
- Future: multi-connection batch runs; encrypted evidence archive; combined
  PDF/HTML evidence report; non-Postgres engines; scheduling.

## Decision log

Chronological record of non-obvious decisions (newest last). Append here when a
decision is made or reversed.

- **Evidence = full-screen screenshots, not the app's rendered view.** Auditors need
  the OS clock in-frame as proof-of-time; the app view can't show it. Video is an
  optional ffmpeg extra.
- **macOS capture via `screencapture`, not kbinani/CGImage.** The deprecated
  CoreGraphics path drops the menu-bar clock/status extras on recent macOS.
- **No stored password; read-only via `PGOPTIONS`.** Secrets stay out of disk;
  read-only is set at connection startup so it can't contaminate CSV output.
- **Stable self-signed signing for macOS dev builds.** Ad-hoc signatures rotate
  their hash each build, so the Screen Recording (TCC) grant won't persist. `make
  dist` re-signs with "PgEvidence Dev"; `scripts/create-signing-cert.sh`
  creates the cert (OpenSSL 3 needs `-legacy` p12 for Apple `security`).
- **PATH fallback for GUI apps.** Finder-launched apps get a minimal PATH, so
  `psql`/`ffmpeg` are resolved from common install dirs too.
- **Run uses on-screen settings.** Start persists the current config first; the UI
  dwell countdown is driven by the backend `run:dwell` event so it matches reality.
- **Uniform checksum naming.** `manifest.json.sha256` (not `run.sha256`) to match
  the `name.ext.sha256` convention.
- **Single sources of truth.** `VERSION` (embedded) for the app version;
  `build/appicon.png` (from `scripts/gen_icon.py`) for the cross-platform icon.
- **Query reorder = drag-and-drop.** Replaced ↑/↓ buttons with native HTML5 DnD.
- **ZIP archiving, ZipCrypto, zip-inside-folder.** Each run can be packaged into a
  `.zip` created *inside* its run folder (folder kept). Encryption is legacy
  ZipCrypto (`yeka/zip` `StandardEncryption`) for compatibility, chosen over AES-256
  despite being weak. Password modes: none / explicit / auto. Explicit password is
  persisted plaintext in `config.json` and the auto password in `<zip>.pwd` — a
  deliberate exception to "no stored password" (which governs DB creds), for
  packaging convenience. Orchestrated from the frontend so the explicit-password
  prompt works.
- **Optional per-query `.sql` + prune-after-zip.** `SaveQuerySQL` (default on)
  writes `NNNN_<slug>.sql` next to each result — the query text, **no checksum**
  (it's reproducible input, not tamper-evidence). `DeleteSourcesAfterZip` (default
  off) removes the loose files after a confirmed archive, keeping only the zip;
  `archive.PruneSources` guards against deleting when the zip is missing/empty.
  `ExcludeVideoFromZip` (default off) keeps `run.mp4` (`archive.VideoName`) out of
  the zip *and* out of the prune (videos are large / poorly compressible), so the
  loose `.mp4` survives even with delete-sources on. `Create`/`PruneSources` take
  the flag; `ArchiveRun`/`ArchiveRunAuto`/`PruneRunDir` read it from config.
- **Generate the full icon set ourselves.** Wails' macOS `.icns` lacks `@1x`
  sizes (generic icon in Finder/Dock/cmd-tab) and it never builds the Windows
  `.ico`. `scripts/gen_icon.py` emits `.png` + complete `.icns` + `.ico`; `make
  dist` swaps the complete `.icns` into the bundle, re-signs, and re-registers
  with Launch Services. (Same issue was hit and fixed in the `osc` project.)

- **Theming = `data-theme` + CSS vars (mirrors `osc`).** `config.Theme`
  (system/light/dark); `theme.ts` sets `<html data-theme>`, resolving `system` via
  `matchMedia` (macOS/Windows) or the Go `IsSystemDark()` gsettings/KDE probe +5s
  poll on Linux (WebKitGTK ignores `prefers-color-scheme`). `color-scheme` per block
  themes native controls. `App.svelte` carries the dark default in `:root`.
- **Run-tab controls write the shared `cfg` store + persist.** Connection dropdown
  and screenshots/video toggles mutate `$cfg` + `SaveConfig`, so Settings (same
  store) stays in sync. Query count lives by the Start button (removed from the tab).
- **Help text behind `Hint` "?" popovers.** Verbose notes (Run idle, ZIP/archive,
  save-on-run, session password) collapsed into hover/focus popovers.
- **Settings auto-save (no Save button).** `on:input`/`on:change` on the settings
  container debounce-save the whole config (the `c = $cfg` object is mutated in
  place, so `SaveConfig($cfg)` writes current values); programmatic mutations
  (add/remove connection, browse) call `autoSave()` too. A transient "Saved ✓" chip
  gives feedback. Run-start `SaveConfig` stays as a safety net.
- **Session password applied on edit (no Set/Clear).** The password field calls
  `SetSessionPassword` on `change` (blur/Enter); empty clears it. Reset/refresh only
  when the *selected connection id* changes (`prevConnId` guard) so unrelated config
  edits don't wipe a typed password. Field is a normal partial-width input (unframed).
- **Editable psql/ffmpeg paths.** `config.PsqlPath`/`FfmpegPath` (blank = auto).
  `psql.SetPath`/`capture.SetFFmpegPath` package overrides, applied via
  `app.applyToolPaths(cfg)` at startup/SaveConfig/StartRun/TestConnection/
  DetectEnvironment. Resolution order: override (if executable) → PATH → common dirs.
  Settings has Browse (`SelectFile` → OpenFileDialog) + auto-detect placeholder.
- **One consistent button system (CSS vars per theme).** primary/active = filled
  accent (`--on-accent` text); default = filled `--bg-3`; ghost & inactive tabs =
  outline (transparent + `--border-strong`); danger = red outline → solid red on
  hover. Hover only via `:not(:disabled):hover` (disabled never react). Native-control
  parity via `color-scheme`; inputs/selects share an explicit height (checkboxes/
  radios excluded so list rows stay compact).
- **WebKit autocapitalize/autocorrect disabled.** WKWebView capitalizes the first
  letter of text inputs by default — wrong for SQL/hosts/paths. Turned off on
  `<body>` and via a `focusin` handler in `main.ts` (covers dynamically added fields).
- **Release pipeline mirrors `osc`.** CI builds DMG (macOS), WiX MSI (Windows),
  deb+rpm (Linux via fpm) on a `vX.Y.Z` tag. WiX MSI chosen over NSIS; Linux packages
  hard-depend on the Postgres client (the app needs `psql`). Ships unsigned. Fresh MSI
  UpgradeCode GUID (never reuse osc's). See the Release / packaging section above.
- **Run-page controls are per-run, not persisted.** Screenshots/Video/ZIP/
  Delete-sources/Exclude-video and the target connection live in an ephemeral
  `runOpts` store, seeded from saved config at app start (load-time sync only).
  They drive the current run but never write back — Settings stays the persisted
  source of truth. `StartRun(screenshots, video, connectionID)` applies them as
  overrides without saving; `ArchiveRun/ArchiveRunAuto/PruneRunDir` take the
  exclude/keep-video flag explicitly (ZIP password policy stays a Settings concern).
  Reverses the earlier "Run controls write the shared cfg + persist" decision.
- **Settings save feedback = flash the Settings tab, not a floating chip.** A
  `savedTick` counter (bumped after each auto-save) drives a two-phase flash in
  `App.svelte`: solid green with the label "Saved" for ~1.5s (hold), then the label
  flips back as the color eases to the active-tab color over ~0.7s (fade). The tab
  uses an invisible sizer span so swapping "Settings"↔"Saved" never resizes it.
- **App-wide no text selection (mirrors `osc`).** `body { user-select: none;
  -webkit-user-select: none }` (the `-webkit-` prefix is required for WebKit/Wails);
  `input, textarea` re-enable it so fields stay editable. Note this also makes the
  checksum/result-preview non-selectable.
- **Window size persistence + Windows console hiding.** See Platform gotchas: save
  the Wails OS window size (not the viewport) to avoid per-launch shrink; run every
  console child through `proc.Hide` so Windows doesn't flash a shell window.
- **Code-review hardening (post-1.0).** From a multi-agent review:
  - *Config is serialized + atomic.* `config` has a package mutex; `Load/Save`
    lock and `Update(mutate)` does a locked read-modify-write; `saveLocked` writes
    temp-file + `os.Rename`. **Window size is owned by `SaveWindowSize`**, so
    `SaveConfig` preserves the on-disk `WindowWidth/Height` (the Settings UI never
    sends them) — otherwise a Settings save with a stale size would revert a resize.
  - *Run tab locks during a run.* Shared `isRunning` store: while a run is active
    the other tab buttons are disabled, so `Run.svelte` can't unmount and drop its
    `run:*` handlers (which would strand the run + re-enable Start → double-run).
    `start()` also sets `running=true` synchronously to guard a double-click.
  - *Run goroutine recovers from panics* (emits `run:done` with the error, releases
    `running`) instead of crashing the app.
  - *CSV integrity:* `psql.RunToFile` closes the output explicitly and checks the
    error (a failed flush = truncated CSV → query fails, not a checksummed lie), and
    `os.Remove`s the file on any error path so no partial `.csv` is left.
- **A cancelled run is not archived.** The runner still writes the partial loose
  files + `manifest.json` (a truthful record of what completed), but the frontend
  skips the auto-ZIP on `done.cancelled` (incomplete evidence) — so prune never runs
  either. The Run summary shows "Run cancelled" / "Partial evidence written to:".
- Plans of record (newest last): build, polish, ZIP archiving, release packaging,
  theming+Run controls+hints, Settings auto-save/password/tool-paths, and per-run
  Run controls + Settings "Saved" flash — under `~/.claude/plans/`.
