---
title: PgEvidence
description: Tamper-evident PostgreSQL audit evidence
---

PgEvidence is a cross-platform desktop app that runs a maintained set of **read-only**
SQL queries against PostgreSQL and produces **tamper-evident evidence** for auditors —
proof of a database's settings, structure, or data at a point in time.

![PgEvidence running a query](../../assets/screenshot-run.png)

## What each run produces

For every query, in order:

- a **CSV** result via the system `psql`,
- a **SHA-256 checksum** sidecar you can verify with `sha256sum -c`,
- a **full-screen screenshot** showing the query, its checksum, and a result preview —
  **with the OS clock in frame** as proof-of-time,
- optionally the query text (`.sql`) and a screen **recording** (`.mp4`),
- a run **`manifest.json`** with its own checksum.

Optionally the whole run is packed into a **ZIP** (optionally password-protected).

## Highlights

| | |
|---|---|
| **Read-only** | Sessions run with `default_transaction_read_only=on` — extracts can't change data |
| **No stored DB password** | Comes from `~/.pgpass` or an in-memory session prompt |
| **Proof-of-time** | The full-screen screenshot captures the OS clock |
| **Reproducible** | `--no-psqlrc`, `ON_ERROR_STOP=1`, checksummed outputs + manifest |
| **Archive** | One ZIP per run; optional password (explicit or auto-generated) |
| **Themes** | System / Light / Dark |

:::note
PgEvidence requires the PostgreSQL client (`psql`) on the machine running the app.
:::

Head to [Installation](/Audit-PG-Extractor/installation/) to get started, or
[Usage](/Audit-PG-Extractor/usage/) for the workflow.
