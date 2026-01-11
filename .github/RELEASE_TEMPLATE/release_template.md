# Release Template

Use this template when creating releases for this project.

## Release Checklist

### Pre-release
- [ ] All tests are passing in CI
- [ ] Version number has been updated (if not using semantic versioning automation)
- [ ] CHANGELOG.md has been updated with new features, fixes, and breaking changes
- [ ] Documentation has been updated to reflect any new features or changes
- [ ] Dependencies have been reviewed and updated if necessary
- [ ] Security vulnerabilities have been addressed

### Release Process
- [ ] Create and push a semantic version tag (e.g., `git tag v1.0.0 && git push origin v1.0.0`)
- [ ] GoReleaser will automatically create the GitHub release
- [ ] Verify that all artifacts were built successfully
- [ ] Verify that checksums are correct
- [ ] Test download and installation of release binaries

### Post-release
- [ ] Announce the release on relevant channels
- [ ] Update documentation sites if applicable
- [ ] Create follow-up issues for any known limitations or future improvements

## Release Notes Template

```markdown
## What's Changed

### 🚀 New Features
- Feature description

### 🐛 Bug Fixes  
- Bug fix description

### 📚 Documentation
- Documentation changes

### 🔧 Maintenance
- Dependency updates
- CI/CD improvements

### ⚠️ Breaking Changes
- Breaking change description and migration guide

## Installation

### Binary Downloads
Download the appropriate binary for your platform from the [releases page](https://github.com/benvon/ai-code-template-go/releases).

### Go Install
```bash
go install github.com/benvon/ai-code-template-go@latest
```

### Homebrew (if available)
```bash
brew install your-formula-name
```

## Docker
```bash
docker pull ghcr.io/benvon/ai-code-template-go:v1.0.0
```

**Full Changelog**: https://github.com/benvon/ai-code-template-go/compare/v0.9.0...v1.0.0
```

## Semantic Versioning Guide

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version when you make incompatible API changes
- **MINOR** version when you add functionality in a backwards compatible manner  
- **PATCH** version when you make backwards compatible bug fixes

### Examples
- `v1.0.0` - Initial stable release
- `v1.1.0` - New backward-compatible features
- `v1.1.1` - Bug fixes
- `v2.0.0` - Breaking changes
