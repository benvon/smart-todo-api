# Release Testing Guide

This guide helps you test the GoReleaser setup locally before pushing tags.

## Prerequisites

Install GoReleaser:
```bash
# macOS
brew install goreleaser

# Linux
curl -sfL https://goreleaser.com/static/run | bash -s -- install

# Or use the one-shot command (as we did)
curl -sfL https://goreleaser.com/static/run | sh -s -- check
```

## Local Testing

### 1. Validate Configuration
```bash
goreleaser check
```

### 2. Test Build (Snapshot Mode)
```bash
# Build without releasing (creates snapshot)
goreleaser build --snapshot --clean

# Check the dist/ folder for generated binaries
ls -la dist/
```

### 3. Test Release (Dry Run)
```bash
# Simulate a release without pushing
goreleaser release --snapshot --clean

# This creates:
# - Binaries in dist/
# - Archives
# - Checksums
# - Release notes
```

## Release Process

### 1. Create a Release Tag
```bash
# Ensure you're on main branch
git checkout main
git pull origin main

# Tag the release
git tag -a v0.1.0 -m "Initial release v0.1.0"

# Push the tag to trigger the release workflow
git push origin v0.1.0
```

### 2. Monitor the Release
- Go to GitHub Actions tab in your repository
- Watch the "Release" workflow
- Check the Releases page for the new release

### 3. Verify Release Artifacts
The release should include:
- Binary archives for each platform
- Checksums file
- Automated changelog
- Release notes

## Supported Platforms

The configuration builds for:
- Linux (amd64, arm64)
- macOS (amd64, arm64) 
- Windows (amd64, arm64)

## Binary Names

Generated binaries follow this pattern:
```
ai-code-template-go_v0.1.0_Darwin_arm64.tar.gz
ai-code-template-go_v0.1.0_Linux_x86_64.tar.gz  
ai-code-template-go_v0.1.0_Windows_x86_64.zip
```

## Troubleshooting

### Common Issues

1. **"No buildable Go source files"**
   - Ensure you have a `main.go` file in the root
   - Check that `go build ./` works locally

2. **"Template execution error"**  
   - Check your `.goreleaser.yml` syntax
   - Run `goreleaser check` locally

3. **"Permission denied" on GitHub**
   - Ensure the workflow has `contents: write` permissions
   - Check that `GITHUB_TOKEN` is properly configured

4. **Missing version information**
   - The ldflags in `.goreleaser.yml` inject version info
   - Test with `./binary --version` after building

### Debug Tips

1. **Test locally first**:
   ```bash
   goreleaser build --snapshot --clean
   ./dist/ai-code-template-go_linux_amd64_v1/ai-code-template-go --version
   ```

2. **Check workflow logs** in GitHub Actions

3. **Validate tag format**: Use semantic versioning (v1.0.0, v1.0.0-beta.1, etc.)

## Version Management

### Semantic Versioning
- `v1.0.0` - Major release
- `v1.1.0` - Minor release  
- `v1.1.1` - Patch release
- `v1.0.0-beta.1` - Pre-release
- `v1.0.0-rc.1` - Release candidate

### Tag Management
```bash
# List all tags
git tag

# Delete a local tag
git tag -d v0.1.0

# Delete a remote tag (be careful!)
git push --delete origin v0.1.0
```
