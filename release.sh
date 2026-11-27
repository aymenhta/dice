
#!/usr/bin/env bash

set -Eeuo pipefail

# -----------------------------------------
# Configuration
# -----------------------------------------

APP_NAME="${APP_NAME:-dice}"
MAIN_PACKAGE="${MAIN_PACKAGE:-./cmd/server}"
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"

BUILD_DIR="${BUILD_DIR:-build}"
DIST_DIR="${DIST_DIR:-dist}"

GO="${GO:-go}"
CGO_ENABLED="${CGO_ENABLED:-0}"

# -----------------------------------------
# Build targets
# -----------------------------------------

TARGETS=(
    "windows amd64"
    "windows arm64"

    "linux amd64"
    "linux arm64"
    "linux riscv64"

    "darwin amd64"
    "darwin arm64"

    "freebsd amd64"
    "freebsd arm64"

    "openbsd amd64"
    "openbsd arm64"

    "netbsd amd64"
    "netbsd arm64"

    "dragonfly amd64"
)

# -----------------------------------------
# Helpers
# -----------------------------------------

log() {
    printf '\033[1;32m[INFO]\033[0m %s\n' "$*"
}

warn() {
    printf '\033[1;33m[WARN]\033[0m %s\n' "$*" >&2
}

die() {
    printf '\033[1;31m[ERROR]\033[0m %s\n' "$*" >&2
    exit 1
}

command -v "$GO" >/dev/null 2>&1 || die "Go is not installed."

GO_VERSION="$("$GO" version)"
log "Using $GO_VERSION"

rm -rf "$BUILD_DIR" "$DIST_DIR"
mkdir -p "$BUILD_DIR" "$DIST_DIR"

# -----------------------------------------
# Build
# -----------------------------------------

SUCCESS=0
FAILED=0

for TARGET in "${TARGETS[@]}"; do
    read -r GOOS GOARCH <<< "$TARGET"

    SUFFIX=""
    [[ "$GOOS" == "windows" ]] && SUFFIX=".exe"

    BINARY="${APP_NAME}${SUFFIX}"
    TARGET_NAME="${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}"

    OUTPUT_DIR="$BUILD_DIR/$TARGET_NAME"
    OUTPUT="$OUTPUT_DIR/$BINARY"

    mkdir -p "$OUTPUT_DIR"

    log "Building $GOOS/$GOARCH"

    if GOOS="$GOOS" \
       GOARCH="$GOARCH" \
       CGO_ENABLED="$CGO_ENABLED" \
       "$GO" build \
           -trimpath \
           -buildvcs=false \
           -ldflags="-s -w -X main.version=$VERSION" \
           -o "$OUTPUT" \
           "$MAIN_PACKAGE"; then

        log "Build successful: $TARGET_NAME"

        # Package the binary.
        if [[ "$GOOS" == "windows" ]]; then
            (
                cd "$BUILD_DIR"
                zip -qr "../$DIST_DIR/$TARGET_NAME.zip" "$TARGET_NAME"
            )
        else
            tar -czf "$DIST_DIR/$TARGET_NAME.tar.gz" \
                -C "$BUILD_DIR" "$TARGET_NAME"
        fi

        ((SUCCESS += 1))
    else
        warn "Build failed: $GOOS/$GOARCH"
        ((FAILED += 1))
        rm -rf "$OUTPUT_DIR"
    fi
done

# -----------------------------------------
# Checksums
# -----------------------------------------

log "Generating SHA-256 checksums"

if command -v sha256sum >/dev/null 2>&1; then
    (
        cd "$DIST_DIR"
        sha256sum * > SHA256SUMS
    )
elif command -v shasum >/dev/null 2>&1; then
    (
        cd "$DIST_DIR"
        shasum -a 256 * > SHA256SUMS
    )
else
    warn "No SHA-256 utility found; skipping checksums."
fi

# -----------------------------------------
# Summary
# -----------------------------------------

echo
log "Release completed"
echo "Version:  $VERSION"
echo "Built:    $SUCCESS"
echo "Failed:   $FAILED"
echo "Output:   $DIST_DIR/"

if (( FAILED > 0 )); then
    exit 1
fi
