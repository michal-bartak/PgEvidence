# PgEvidence — build helpers.
#
# On macOS, screenshots need Screen Recording permission, which only persists
# across rebuilds if the app is signed with a STABLE identity. `make dist` builds
# and then re-signs with the self-signed "PgEvidence Dev" cert created by
# scripts/create-signing-cert.sh.

APP := build/bin/pgevidence.app
ICNS := build/appicon.icns
SIGN_IDENTITY ?= PgEvidence Dev
LSREGISTER := /System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister

.PHONY: build sign dist dev cert reset-screen-perm vet icon fix-icon

build:
	wails build

# The .icns Wails generates omits @1x sizes, so macOS shows a generic icon.
# Replace it with our complete one (scripts/gen_icon.py) before signing.
fix-icon:
	@[ -f "$(ICNS)" ] && cp "$(ICNS)" "$(APP)/Contents/Resources/iconfile.icns" && echo "replaced bundle icns" || echo "WARN: $(ICNS) missing — run 'make icon'"

# Re-sign the built .app with the stable self-signed identity (macOS only).
# Sign AFTER replacing the icns, or the signature won't cover it.
sign: fix-icon
	codesign --force --deep --sign "$(SIGN_IDENTITY)" "$(APP)"
	@codesign -dvv "$(APP)" 2>&1 | grep -E "Authority|Identifier" || true

# Production build that keeps a stable signing identity → stable TCC permission,
# and a correct icon (re-registered so Finder/Dock/cmd-tab refresh).
dist: build sign
	@touch "$(APP)"
	@"$(LSREGISTER)" -f "$(APP)" 2>/dev/null || true
	@echo "Built and signed: $(APP)"

dev:
	wails dev

# One-time: create the local self-signed signing certificate.
cert:
	./scripts/create-signing-cert.sh

# Clear any stale Screen Recording grant so the freshly signed app can be granted.
reset-screen-perm:
	tccutil reset ScreenCapture com.wails.pgevidence || true

vet:
	go vet ./...

# Regenerate the icon in all forms: build/appicon.png, build/appicon.icns,
# build/windows/icon.ico (requires Python + Pillow; .icns also needs iconutil).
icon:
	python3 scripts/gen_icon.py
