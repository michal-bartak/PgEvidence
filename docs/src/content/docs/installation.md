---
title: Installation
description: Download and install PgEvidence on macOS, Windows, or Linux
---

Download the latest installer from the
[Releases page](https://github.com/michal-bartak/PgEvidence/releases).

## Prerequisite: psql

PgEvidence runs the system **`psql`** (PostgreSQL client). Install it if you don't have it
(Homebrew, Postgres.app, the EDB installer, or your distro's `postgresql-client`). The Linux
packages pull it in automatically. If `psql` lives somewhere unusual, set its path in
**Settings → Environment**.

## macOS

1. Open the `.dmg` and drag **PgEvidence** to **Applications**, then eject the disk image.
2. The app is unsigned, so Gatekeeper blocks it on first launch. Either:
   - download the DMG with `curl -LJO <url>` (no quarantine flag is set), **or**
   - run once: `xattr -d com.apple.quarantine /Applications/pgevidence.app`
3. Grant **Screen Recording** (System Settings → Privacy & Security → Screen Recording) so the
   proof-of-time screenshots work, then **quit and reopen** the app.

## Windows

Run the `.msi`. SmartScreen may warn — choose **More info → Run anyway**.

## Linux

```bash
sudo apt install ./pgevidence-*-linux-amd64.deb    # Debian / Ubuntu
sudo dnf install ./pgevidence-*-linux-amd64.rpm    # Fedora / RHEL
```

The PostgreSQL client is installed as a dependency.
