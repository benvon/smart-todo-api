#!/usr/bin/env bash

set -euo pipefail

# Package release script for SPA/PWA deployment
# This script builds the frontend and creates a deployment package

VERSION="${1:-unknown}"
PACKAGE_NAME="smart-todo-web-${VERSION}"
PACKAGE_DIR="/tmp/${PACKAGE_NAME}"

echo "Building frontend for version ${VERSION}..."

# Clean and create package directory
rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}"

# Build the frontend
cd "$(dirname "$0")/.."
npm ci
npm run build

# Copy built files (config.json is intentionally excluded—it is deployment-specific
# and must be supplied at deploy time, not bundled in the release tarball)
echo "Copying files to package directory..."
cp -r dist "${PACKAGE_DIR}/"
cp index.html "${PACKAGE_DIR}/"
cp app.html "${PACKAGE_DIR}/"
cp -r css "${PACKAGE_DIR}/"
cp manifest.json "${PACKAGE_DIR}/"
cp nginx.conf "${PACKAGE_DIR}/"

# Create VERSION file
echo "${VERSION}" > "${PACKAGE_DIR}/VERSION"

# Create README for deployment
cat > "${PACKAGE_DIR}/README.md" << EOF
# Smart Todo Web Frontend - ${VERSION}

## Deployment Instructions

This package contains the built frontend for Smart Todo API.

### Contents

- \`dist/\` - Built JavaScript bundles
- \`index.html\` - Login page
- \`app.html\` - Main application page
- \`css/\` - Stylesheets
- \`manifest.json\` - PWA manifest
- \`nginx.conf\` - Example nginx configuration
- \`VERSION\` - Version information

### Cloudflare Pages Deployment

1. Extract this package
2. Upload the contents to Cloudflare Pages
3. Set the build output directory to the root of the extracted package
4. Configure environment variables as needed

For automated deployment from CI, see the main project’s docs/DEPLOYING_FRONTEND.md.

### Nginx Deployment

1. Extract this package to your web server directory
2. Use the included \`nginx.conf\` as a reference
3. Configure your nginx server to serve the static files
4. Ensure proper routing for SPA (all routes should serve index.html)

### Environment Configuration

The frontend requires a \`config.json\` file with the API base URL.
Create this file in the root of the deployment directory:

\`\`\`json
{
  "apiBaseUrl": "https://your-api-domain.com"
}
\`\`\`

For more information, see the main project README.
EOF

# Ensure config.json is not in the package (guard against future accidental inclusion)
rm -f "${PACKAGE_DIR}/config.json"

# Create tarball
echo "Creating tarball..."
cd /tmp
tar -czf "${PACKAGE_NAME}.tar.gz" "${PACKAGE_NAME}"

# Output the package path (absolute path for GitHub Actions)
# Only output the path to stdout (last line) for workflow extraction
# Send informational messages to stderr
PACKAGE_FILE="/tmp/${PACKAGE_NAME}.tar.gz"
echo "Package created: ${PACKAGE_FILE}" >&2
echo "${PACKAGE_FILE}"
