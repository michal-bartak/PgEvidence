# Audit Extractor — build helpers.
#
# On macOS, screenshots need Screen Recording permission, which only persists
# across rebuilds if the app is signed with a STABLE identity. `make dist` builds
# and then re-signs with the self-signed "Audit Extractor Dev" cert created by
# scripts/create-signing-cert.sh.

APP := build/bin/audit-extractor.app
SIGN_IDENTITY ?= Audit Extractor Dev

.PHONY: build sign dist dev cert reset-screen-perm vet icon

build:
	wails build

# Re-sign the built .app with the stable self-signed identity (macOS only).
sign:
	codesign --force --deep --sign "$(SIGN_IDENTITY)" "$(APP)"
	@codesign -dvv "$(APP)" 2>&1 | grep -E "Authority|Identifier" || true

# Production build that keeps a stable signing identity → stable TCC permission.
dist: build sign
	@echo "Built and signed: $(APP)"

dev:
	wails dev

# One-time: create the local self-signed signing certificate.
cert:
	./scripts/create-signing-cert.sh

# Clear any stale Screen Recording grant so the freshly signed app can be granted.
reset-screen-perm:
	tccutil reset ScreenCapture com.wails.audit-extractor || true

vet:
	go vet ./...

# Regenerate build/appicon.png from scripts/gen_icon.py (requires Python + Pillow).
icon:
	python3 scripts/gen_icon.py
