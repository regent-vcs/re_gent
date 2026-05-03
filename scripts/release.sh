#!/bin/bash
set -e

# Regent Release Script
# Usage: ./scripts/release.sh [version]
# Example: ./scripts/release.sh v0.2.0

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.2.0"
    exit 1
fi

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z]+)?$ ]]; then
    echo "Error: Version must be in format vX.Y.Z or vX.Y.Z-suffix (e.g., v0.2.0 or v0.2.0-beta)"
    exit 1
fi

echo "🔍 Pre-flight checks..."

# Check if on main or develop branch
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$BRANCH" != "main" && "$BRANCH" != "develop" ]]; then
    echo "⚠️  Warning: You're on branch '$BRANCH', not 'main' or 'develop'"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "❌ Error: You have uncommitted changes"
    git status --short
    exit 1
fi

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "❌ Error: Tag $VERSION already exists"
    exit 1
fi

# Run tests
echo "🧪 Running tests..."
go test ./... || {
    echo "❌ Tests failed"
    exit 1
}

# Run linter
echo "🔍 Running linter..."
golangci-lint run --timeout=5m || {
    echo "❌ Linting failed"
    exit 1
}

# Build binary to verify
echo "🔨 Building binary..."
go build -o rgt ./cmd/rgt || {
    echo "❌ Build failed"
    exit 1
}

# Test the binary
echo "✅ Testing binary..."
./rgt version || {
    echo "❌ Binary test failed"
    exit 1
}

echo ""
echo "📋 Release Summary:"
echo "  Version: $VERSION"
echo "  Branch:  $BRANCH"
echo "  Commit:  $(git rev-parse --short HEAD)"
echo ""

read -p "Create release tag and push? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled"
    exit 0
fi

# Create annotated tag
echo "🏷️  Creating tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

# Push tag
echo "📤 Pushing tag to GitHub..."
git push origin "$VERSION"

echo ""
echo "✅ Release initiated!"
echo ""
echo "GitHub Actions will now:"
echo "  1. Build binaries for all platforms"
echo "  2. Create a GitHub Release"
echo "  3. Upload artifacts"
echo "  4. Update Homebrew tap (if configured)"
echo ""
echo "Monitor progress at:"
echo "  https://github.com/regent-vcs/re_gent/actions"
echo ""
echo "Release will be available at:"
echo "  https://github.com/regent-vcs/re_gent/releases/tag/$VERSION"
