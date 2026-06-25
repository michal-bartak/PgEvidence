# Audit Extractor

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

## Prerequisites

- **`psql`** (PostgreSQL client) on your PATH — **required**.
- **`ffmpeg`** — optional, only if you enable video recording.
- **macOS only:** grant the app **Screen Recording** permission
  (System Settings → Privacy & Security → Screen Recording), otherwise screenshots
  come out blank. The app warns you if it captures a blank image.

## Usage

1. **Settings** tab — define your database connection (host/port/db/user/sslmode),
   choose the output folder, dwell time, and capture options. Optionally enter a
   session password (kept in memory only) or rely on `~/.pgpass`. Use **Test
   connection** to confirm.
2. **Queries** tab — add/edit/remove queries one by one, **drag the ⠿ handle to
   reorder** them, or **Import all** to paste a JSON query set or a plain `.sql`
   script (split on semicolons). When importing a script, the free text before each
   query — a plain-text description and/or `--` comment — becomes its name and is
   excluded from the SQL; the query is taken to start at the first SQL keyword
   (`SELECT`, `WITH`, …). With no leading text, the name falls back to the first SQL
   line. **Export all** saves your set as JSON.
3. **Run** tab — **Start run**. Each query is executed in order; you'll see it on
   screen with its checksum and result preview, and the evidence folder opens when
   done.

## Security & integrity

- **No passwords are stored** — connection definitions hold no secret; the password
  lives only in memory for the session or comes from `~/.pgpass`.
- **Read-only enforcement** is on by default (the session runs with
  `default_transaction_read_only=on`), so an extract query can't mutate data.
- Runs use `--no-psqlrc` and `ON_ERROR_STOP=1` for reproducibility, and every
  output is checksummed.

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
