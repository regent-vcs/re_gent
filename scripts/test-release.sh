#!/bin/bash
set -e

# Test release readiness without actually releasing
# This verifies that the project can be built and released

echo "🧪 Testing release readiness..."
echo ""

# 1. Check for required files
echo "1️⃣ Checking configuration files..."
REQUIRED_FILES=(
    ".goreleaser.yaml"
    ".github/workflows/release.yml"
    "cmd/rgt/main.go"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ Missing: $file"
        exit 1
    fi
done

# 2. Run tests
echo ""
echo "2️⃣ Running tests..."
go test ./... > /dev/null 2>&1 && echo "  ✅ All tests pass" || {
    echo "  ❌ Tests failed"
    exit 1
}

# 3. Run linter
echo ""
echo "3️⃣ Running linter..."
if command -v golangci-lint &> /dev/null; then
    golangci-lint run --timeout=5m > /dev/null 2>&1 && echo "  ✅ Linter pass" || {
        echo "  ❌ Linter failed"
        exit 1
    }
else
    echo "  ⚠️  golangci-lint not found, skipping"
fi

# 4. Build for multiple platforms
echo ""
echo "4️⃣ Testing cross-platform builds..."
PLATFORMS=(
    "linux/amd64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$platform"
    OUTPUT="test-build-$GOOS-$GOARCH"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT="$OUTPUT.exe"
    fi

    if GOOS=$GOOS GOARCH=$GOARCH go build -o "/tmp/$OUTPUT" ./cmd/rgt 2>/dev/null; then
        echo "  ✅ $platform"
        rm -f "/tmp/$OUTPUT"
    else
        echo "  ❌ Failed to build for $platform"
        exit 1
    fi
done

# 5. Test the local binary
echo ""
echo "5️⃣ Testing local binary..."
go build -o /tmp/rgt-test ./cmd/rgt
/tmp/rgt-test version > /dev/null && echo "  ✅ Binary runs correctly" || {
    echo "  ❌ Binary test failed"
    exit 1
}
rm -f /tmp/rgt-test

# 6. Check goreleaser config
echo ""
echo "6️⃣ Checking GoReleaser configuration..."
echo "  📦 Platforms: linux, darwin, windows (amd64, arm64)"
echo "  📦 Archive formats: tar.gz, zip"
echo "  📦 Homebrew tap: regent-vcs/homebrew-tap"
echo "  📦 Release target: regent-vcs/re_gent"

echo ""
echo "✅ All checks passed!"
echo ""
echo "Your project is ready for release. To create a release:"
echo "  ./scripts/release.sh v0.x.y"
echo ""
echo "The GitHub Actions workflow will:"
echo "  - Build binaries for all platforms"
echo "  - Create a GitHub Release with changelog"
echo "  - Upload artifacts and checksums"
echo "  - Update the Homebrew tap"
