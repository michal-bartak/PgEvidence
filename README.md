# PgEvidence

A cross-platform desktop app for running ad-hoc PostgreSQL extracts for auditors —
and producing **reproducible, tamper-evident evidence** that each query was run and
what it returned.

For every query in your set, the app:

1. runs it through the system `psql` and saves the result as **CSV**,
2. computes a **SHA-256 checksum** (sha256sum-compatible sidecar file),
3. shows the query, its checksum, and a preview of the result on screen for a
   configurable dwell time, and takes a **full-screen screenshot** — capturing the
   OS clock in-frame as proof of when it ran,
4. writes a run **manifest** with a checksum sidecar over it.

Result and checksum (and screenshot) for a query share a common filename stem.

## Install

Download the latest installer from the
[Releases page](https://github.com/michal-bartak/Audit-PG-Extractor/releases):

- **macOS** — `…-macos-universal.dmg`: open it, drag **PgEvidence** to
  Applications. The app is unsigned, so on first launch either download the DMG via
  `curl -LJO <url>` (no quarantine) or run
  `xattr -d com.apple.quarantine /Applications/pgevidence.app`. Then grant
  Screen Recording permission and reopen.
- **Windows** — `…-windows-amd64.msi`: run it (SmartScreen → More info → Run anyway).
- **Linux** — `sudo apt install ./…-linux-amd64.deb` (Debian/Ubuntu) or
  `sudo dnf install ./…-linux-amd64.rpm` (Fedora/RHEL); the Postgres client is pulled
  in automatically.

All builds require **`psql`** at runtime (see Prerequisites). Releases are produced
by the `Release` GitHub Actions workflow (manual dispatch on a `vX.Y.Z` tag).

## Evidence layout

One run writes a timestamped folder under your chosen output directory:

```
audit-run-20260625-153000/
  0001_active_users.sql          # the query text (if "Save each query as .sql" is on; no checksum)
  0001_active_users.csv
  0001_active_users.csv.sha256
  0001_active_users.png          # full-screen screenshot (incl. OS clock)
  0002_orders_last_month.csv
  0002_orders_last_month.csv.sha256
  0002_orders_last_month.png
  manifest.json                  # queries, files, checksums, timings, versions
  manifest.json.sha256           # checksum over manifest.json
  run.mp4                        # only if video recording was enabled
  audit-run-20260625-153000.zip  # archive of the above (if archiving is enabled)
  audit-run-20260625-153000.zip.pwd  # generated password (auto mode only)
```

Auditors can verify any result independently:

```
cd audit-run-20260625-153000
sha256sum -c 0001_active_users.csv.sha256
```

## Archiving

When **Create a ZIP archive** is enabled (Settings → Archive), each run is packaged
into a `.zip` placed inside its run folder (the loose files are kept too). Password
protection:

- **None** — plain ZIP.
- **Explicit** — encrypt with the password from Settings; if left blank there, the
  app prompts for one after the run (cancel to skip archiving).
- **Auto-generated** — a random password is created and written next to the archive
  as `<name>.zip.pwd`.

Encryption uses legacy **ZipCrypto** for compatibility — it opens with macOS
`unzip` (which prompts for the password), Windows Explorer, 7-Zip, etc. Note it is
cryptographically weak, and passwords are stored in plaintext (the explicit one in
the config file, the auto one in the `.pwd` sidecar) — this is packaging
convenience, not strong secrecy.

Optionally, **Delete source files after a successful ZIP** (Settings → Archive)
removes the loose files once the archive is written, leaving only the `.zip` (and
`.pwd`) in the run folder. It only runs after the archive is confirmed present.

**Exclude the video from the ZIP** (Settings → Archive) keeps the recording
(`run.mp4`) out of the archive — useful because videos are large and compress
poorly. When set, the `.mp4` is left in the run folder even if *Delete source
files* is on (so it isn't lost), while everything else still goes into the zip.

## Prerequisites

- **`psql`** (PostgreSQL client) on your PATH — **required**.
- **`ffmpeg`** — optional, only if you enable video recording.
- **macOS only:** grant the app **Screen Recording** permission
  (System Settings → Privacy & Security → Screen Recording), otherwise screenshots
  come out blank. The app warns you if it captures a blank image.

## Usage

1. **Settings** tab — define your database connection(s) (host/port/db/user/sslmode),
   choose the output folder, dwell time, and capture options. **Changes save
   automatically** (no Save button). Optionally type a session password (kept in
   memory only, applied when you leave the field; empty it to clear) or rely on
   `~/.pgpass`, and use **Test connection** to confirm. You can also pick the
   **theme** (System / Light / Dark) and point the app at a custom **psql** or
   **ffmpeg** binary (blank = auto-detect).
2. **Queries** tab — add/edit/remove queries one by one, **drag the ⠿ handle to
   reorder** them, or **Import all** to paste a JSON query set or a plain `.sql`
   script (split on semicolons). When importing a script, the free text before each
   query — a plain-text description and/or `--` comment — becomes its name and is
   excluded from the SQL; the query is taken to start at the first SQL keyword
   (`SELECT`, `WITH`, …). With no leading text, the name falls back to the first SQL
   line. **Export all** saves your set as JSON.
3. **Run** tab — pick the connection from the dropdown, toggle **Screenshots**/
   **Video** as needed (these also update Settings), then **Start run**. Each query
   runs in order; you'll see it on screen with its checksum and result preview, and
   the evidence folder opens when done.

## Security & integrity

- **No passwords are stored** — connection definitions hold no secret; the password
  lives only in memory for the session or comes from `~/.pgpass`.
- **Read-only enforcement** is on by default (the session runs with
  `default_transaction_read_only=on`), so an extract query can't mutate data.
- Runs use `--no-psqlrc` and `ON_ERROR_STOP=1` for reproducibility, and every
  output is checksummed.

## Troubleshooting

**macOS keeps prompting "PgEvidence is recording your screen" even though I granted it.**
That dialog is macOS's built‑in screen‑recording *reminder* (Sequoia and later), separate
from the Screen Recording toggle in System Settings. macOS shows it on first use after an
app is installed/updated, then periodically — even for apps that already have permission.
It only affects **video recording** (which uses a continuous capture stream); screenshots
use Apple's `screencapture` tool and don't trigger it. Because capture is already permitted,
recording starts immediately, so the reminder can appear in the first seconds of the video.
Click **Allow**; it won't reappear until the next update or month. (We don't record audio —
ffmpeg captures video only; the "and audio" wording is macOS's.)

**Screenshots come out blank (macOS).** Grant **Screen Recording** to PgEvidence in
System Settings → Privacy & Security → Screen Recording, then **quit and reopen** the app
(a TCC grant doesn't apply to an already‑running process).

**macOS won't open the app ("unidentified developer").** The app is unsigned (Gatekeeper).
Either download the DMG with `curl -LJO <url>` (no quarantine flag is set) or run once:
`xattr -d com.apple.quarantine /Applications/pgevidence.app`.

**"psql not found".** Install the PostgreSQL client (`psql`). PgEvidence auto‑detects it on
PATH and in common locations (Homebrew, Postgres.app, EDB, `Program Files`); if it's
elsewhere, set the path in **Settings → Environment → psql path** (Browse…). The Linux
packages install the client automatically.

**Linux: sluggish UI or wrong monitor.** The provided packages launch under XWayland
(`GDK_BACKEND=x11`) automatically. If you run the binary directly, prefix it:
`GDK_BACKEND=x11 pgevidence`.

## Development

```sh
wails dev      # live development with hot reload
wails build    # production bundle -> build/bin/
make icon      # regenerate the app icon (build/appicon.png) from scripts/gen_icon.py
```

The app version is shown in the header and Settings → Environment, and recorded in
every manifest. It comes from the repo-root `VERSION` file — bump it there.

### macOS: make Screen Recording permission stick

`wails build` ad-hoc signs the app, and that signature changes on every build, so
macOS forgets the Screen Recording grant each time. To keep it:

```sh
make cert              # once per machine: create a self-signed signing cert
make dist              # build + sign with a stable identity
make reset-screen-perm # clear any stale grant, then open the app and grant once
```

After that, future `make dist` builds keep the permission.

See [CLAUDE.md](CLAUDE.md) for architecture, the package map, and contributor notes.
