---
title: Building from source
description: Build PgEvidence locally and cut releases
---

Requires Go, Node.js, and the [Wails v2](https://wails.io) CLI.

```bash
wails dev      # live development (Vite HMR + Go)
wails build    # production bundle -> build/bin/
make icon      # regenerate the icon: build/appicon.png + .icns + .ico
```

The app version is the single `VERSION` file at the repo root, embedded at build time.

## macOS: stable signing

`wails build` ad-hoc signs the app, and that signature changes every build, so macOS forgets
the Screen Recording grant each time. Sign with a stable self-signed identity instead:

```bash
make cert   # once per machine: create the "PgEvidence Dev" cert
make dist   # build + re-sign with the stable identity (keeps the permission)
```

## Releases

`.github/workflows/release.yml` (manual **workflow_dispatch** on a `vX.Y.Z` tag) builds the
macOS **DMG**, Windows **MSI**, and Linux **deb/rpm** in one matrix. The tag must equal
`v$(cat VERSION)`.

See `CLAUDE.md` in the repository for architecture and the full decision log.
