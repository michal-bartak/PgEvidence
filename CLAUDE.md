# CLAUDE.md — Audit Extractor

Developer/agent notes for this repo. Read this before changing code. **This file
is the maintained in-repo design record — keep the Decision log (bottom) and these
notes current when decisions change.**

## What this is

A cross-platform desktop app (Go + Wails v2, Svelte-TS frontend) that runs a
maintained set of SQL queries against PostgreSQL one-by-one and produces
**tamper-evident audit evidence** for each query:

- a CSV result file (`NNNN_<slug>.csv`) via the system `psql --csv`,
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
- **App icon has one source:** `build/appicon.png`, generated from
  `scripts/gen_icon.py` (mirrors `build/appicon.svg`). Wails derives all platform
  icons from it at build time.

## Architecture / package map

Go backend drives the run loop and emits Wails runtime events; the Svelte frontend
is a thin live view. `App` (in `app.go`) is the Wails-bound object and implements
`runner.UI`.

```
main.go                      Wails bootstrap (window 1200x820, bindings)
app.go                       App struct: bound methods + runner.UI (Emit/BringToFront)
internal/config/             Config + Connection list; JSON in os.UserConfigDir()/audit-extractor
internal/store/              Query CRUD: Upsert/Delete/Move/ReplaceAll/Import/Export
                             Import parses JSON set OR splits a SQL script on top-level ';'
internal/psql/               Runs system psql (--csv, --no-psqlrc, ON_ERROR_STOP=1) to a file
internal/checksum/           SHA-256 + WriteSidecar (coreutils "<hex>  <name>" format)
internal/capture/            capture.go: full-display screenshot (kbinani/screenshot)
                             recorder.go: optional ffmpeg recorder (experimental, per-OS input)
internal/manifest/           Manifest struct + Write (manifest.json + manifest.json.sha256)
internal/runner/             Orchestrates the run: loop, run:* events, dwell, screenshot, manifest
frontend/src/App.svelte      Shell: tabs (Queries/Run/Settings), loads env+config+queries on mount
frontend/src/stores.ts       Svelte stores (cfg, queries, env, activeTab) + event payload types
frontend/src/views/          Queries.svelte, Run.svelte, Settings.svelte
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

**macOS signing (why `make dist`, not `wails build`):** `wails build` ad-hoc
signs, and an ad-hoc signature's hash changes every build, so the Screen
Recording (TCC) grant does NOT persist across rebuilds. Signing with a stable
self-signed identity ("Audit Extractor Dev", created by
`scripts/create-signing-cert.sh`) makes the grant stick. `make dist` builds then
`codesign --force --deep --sign "Audit Extractor Dev"`. After the FIRST signed
build, run `make reset-screen-perm` and grant once; subsequent signed builds keep
it. (`security find-identity -v` shows 0 valid identities — expected, the cert is
untrusted; codesign still signs and TCC keys on the signing identity, not trust.)

Module path is `audit-extractor`; internal packages import as `audit-extractor/internal/...`.

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
  dist` re-signs with "Audit Extractor Dev"; `scripts/create-signing-cert.sh`
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

- Plans of record: `~/.claude/plans/resilient-toasting-lighthouse.md` (build),
  `~/.claude/plans/1-store-your-memory-woolly-star.md` (polish pass).
