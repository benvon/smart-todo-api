#!/bin/sh
set -e

PLUGIN_NAME="rabbitmq_delayed_message_exchange"

# Plugin version compatibility:
# - rabbitmq:3-management-alpine currently contains RabbitMQ 3.13.7
# - Plugin version 3.13.0 is the correct and latest version for RabbitMQ 3.13.x series
# - Plugin versions must match RabbitMQ major.minor version series
# Reference: https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases
DEFAULT_PLUGIN_VERSION="3.13.0"
PLUGIN_VERSION="${DEFAULT_PLUGIN_VERSION}"

# Find the actual plugin directory
# In Alpine image, /plugins is typically a symlink to /opt/rabbitmq/plugins
# RabbitMQ uses /plugins as the standard location
PLUGINS_DIR="/plugins"

# If /plugins is a symlink, follow it to get the actual directory
if [ -L "${PLUGINS_DIR}" ]; then
    REAL_DIR=$(readlink -f "${PLUGINS_DIR}" || readlink "${PLUGINS_DIR}")
    echo "Found ${PLUGINS_DIR} symlink pointing to: ${REAL_DIR}"
    # Use the real directory for downloading, but RabbitMQ will find it via the symlink
    DOWNLOAD_DIR="${REAL_DIR}"
else
    # If /plugins doesn't exist yet, use /opt/rabbitmq/plugins (standard location)
    DOWNLOAD_DIR="/opt/rabbitmq/plugins"
    # Create /plugins symlink if it doesn't exist
    if [ ! -e "${PLUGINS_DIR}" ]; then
        mkdir -p "${DOWNLOAD_DIR}"
        ln -s "${DOWNLOAD_DIR}" "${PLUGINS_DIR}" || true
        echo "Created symlink ${PLUGINS_DIR} -> ${DOWNLOAD_DIR}"
    fi
fi

# Ensure plugins directory exists and is writable
mkdir -p "${DOWNLOAD_DIR}"
echo "Using plugin directory: ${DOWNLOAD_DIR} (RabbitMQ sees: ${PLUGINS_DIR})"

# Install wget if not available (needed for downloading plugin)
if ! command -v wget > /dev/null 2>&1; then
    echo "wget not found, installing..."
    if command -v apk > /dev/null 2>&1; then
        apk add --no-cache wget || {
            echo "Failed to install wget with apk"
            # Try curl as alternative
            if ! command -v curl > /dev/null 2>&1; then
                apk add --no-cache curl || {
                    echo "Failed to install curl with apk"
                    echo "Cannot download plugin without wget or curl"
                    exit 1
                }
            fi
        }
    else
        echo "Cannot install wget: apk package manager not available"
        if ! command -v curl > /dev/null 2>&1; then
            echo "Neither wget nor curl available, cannot download plugin"
            exit 1
        fi
    fi
fi

# Start RabbitMQ in background first
/usr/local/bin/docker-entrypoint.sh rabbitmq-server -detached

# Wait for RabbitMQ to be ready
echo "Waiting for RabbitMQ to start..."
for i in $(seq 1 60); do
    if rabbitmq-diagnostics -q ping > /dev/null 2>&1; then
        echo "RabbitMQ is ready!"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "Error: RabbitMQ did not start in time"
        exit 1
    fi
    sleep 1
done

# Detect RabbitMQ version to download matching plugin
echo "Detecting RabbitMQ version..."
# Try multiple methods to detect version
RABBITMQ_VERSION=""
if command -v rabbitmqctl > /dev/null 2>&1; then
    RABBITMQ_VERSION=$(rabbitmqctl version 2>/dev/null | grep -oE 'RabbitMQ[^[:space:]]*' | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo "")
fi
if [ -z "$RABBITMQ_VERSION" ] && command -v rabbitmq-diagnostics > /dev/null 2>&1; then
    RABBITMQ_VERSION=$(rabbitmq-diagnostics server_version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 | cut -d. -f1-2 || echo "")
fi
if [ -z "$RABBITMQ_VERSION" ]; then
    # Try to get version from environment
    RABBITMQ_VERSION=$(rabbitmqctl environment 2>/dev/null | grep -i 'rabbit.*version' | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo "")
fi

if [ -n "$RABBITMQ_VERSION" ]; then
    echo "RabbitMQ version detected: ${RABBITMQ_VERSION}"
else
    echo "Could not detect RabbitMQ version, will use default plugin version"
fi

# Map RabbitMQ version to plugin version
# Plugin versions must match RabbitMQ major.minor version series
# Plugin versions: https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases
if echo "$RABBITMQ_VERSION" | grep -qE "^4\.[0-9]"; then
    # RabbitMQ 4.x requires plugin 4.x
    PLUGIN_VERSION="4.2.0"
    echo "Detected RabbitMQ 4.x, using plugin version ${PLUGIN_VERSION}"
elif echo "$RABBITMQ_VERSION" | grep -qE "^3\.13"; then
    # RabbitMQ 3.13.x requires plugin 3.13.0 (latest for 3.13 series)
    PLUGIN_VERSION="3.13.0"
    echo "Detected RabbitMQ 3.13.x, using plugin version ${PLUGIN_VERSION}"
elif echo "$RABBITMQ_VERSION" | grep -qE "^3\.12"; then
    # RabbitMQ 3.12.x requires plugin 3.12.0
    PLUGIN_VERSION="3.12.0"
    echo "Detected RabbitMQ 3.12.x, using plugin version ${PLUGIN_VERSION}"
elif echo "$RABBITMQ_VERSION" | grep -qE "^3\.11"; then
    # RabbitMQ 3.11.x requires plugin 3.11.1
    PLUGIN_VERSION="3.11.1"
    echo "Detected RabbitMQ 3.11.x, using plugin version ${PLUGIN_VERSION}"
else
    # Default to 3.13.0 for rabbitmq:3-management-alpine (which is 3.13.7)
    PLUGIN_VERSION="${DEFAULT_PLUGIN_VERSION}"
    echo "Could not detect RabbitMQ version, using default plugin version: ${PLUGIN_VERSION}"
    echo "This default works with rabbitmq:3-management-alpine (RabbitMQ 3.13.x)"
fi

PLUGIN_FILE="${PLUGIN_NAME}-${PLUGIN_VERSION}.ez"
echo "Using plugin version: ${PLUGIN_VERSION} (file: ${PLUGIN_FILE})"

# Get the actual plugin directories RabbitMQ is using
echo "Checking RabbitMQ plugin directories..."
PLUGIN_DIRS_OUTPUT=$(rabbitmq-plugins directories -s 2>/dev/null || echo "")
echo "$PLUGIN_DIRS_OUTPUT"

# Extract the plugin archives directory from the output
ACTUAL_PLUGIN_DIR=$(echo "$PLUGIN_DIRS_OUTPUT" | grep -i "plugin.*archive" | grep -oE '/[^[:space:]]+' | head -1 || echo "")
if [ -z "$ACTUAL_PLUGIN_DIR" ]; then
    # Fallback: use the standard location
    ACTUAL_PLUGIN_DIR="${DOWNLOAD_DIR}"
fi
echo "RabbitMQ plugin archives directory: ${ACTUAL_PLUGIN_DIR}"

# Download plugin if it doesn't exist
PLUGIN_TARGET="${ACTUAL_PLUGIN_DIR}/${PLUGIN_FILE}"
if [ ! -f "${PLUGIN_TARGET}" ]; then
    echo "Plugin file does not exist at: ${PLUGIN_TARGET}"
    echo "Downloading ${PLUGIN_NAME} plugin version ${PLUGIN_VERSION}..."
    
    # GitHub release URLs use "v" prefix before version number
    PLUGIN_URL="https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases/download/v${PLUGIN_VERSION}/${PLUGIN_FILE}"
    echo "Download URL: ${PLUGIN_URL}"
    
    # Ensure directory exists
    mkdir -p "${ACTUAL_PLUGIN_DIR}"
    
    # Try wget first, fall back to curl if wget is not available
    DOWNLOAD_SUCCESS=false
    if command -v wget > /dev/null 2>&1; then
        echo "Attempting download with wget..."
        # Use simpler wget command without progress bar (more compatible)
        if wget -q "${PLUGIN_URL}" -O "${PLUGIN_TARGET}" 2>&1; then
            echo "Plugin downloaded successfully with wget to ${PLUGIN_TARGET}"
            DOWNLOAD_SUCCESS=true
        else
            echo "ERROR: Failed to download plugin with wget"
            rm -f "${PLUGIN_TARGET}" 2>/dev/null || true
        fi
    fi
    
    if [ "$DOWNLOAD_SUCCESS" = false ] && command -v curl > /dev/null 2>&1; then
        echo "Attempting download with curl..."
        if curl -sfL "${PLUGIN_URL}" -o "${PLUGIN_TARGET}"; then
            echo "Plugin downloaded successfully with curl to ${PLUGIN_TARGET}"
            DOWNLOAD_SUCCESS=true
        else
            echo "ERROR: Failed to download plugin with curl"
            rm -f "${PLUGIN_TARGET}" 2>/dev/null || true
        fi
    fi
    
    if [ "$DOWNLOAD_SUCCESS" = false ]; then
        echo "FATAL: Failed to download plugin. Neither wget nor curl succeeded."
        echo "Please check network connectivity and the plugin URL."
        exit 1
    fi
    
    # Verify file was downloaded and has content
    if [ ! -f "${PLUGIN_TARGET}" ]; then
        echo "FATAL: Plugin file was not created at ${PLUGIN_TARGET}"
        exit 1
    fi
    
    FILE_SIZE=$(stat -f%z "${PLUGIN_TARGET}" 2>/dev/null || stat -c%s "${PLUGIN_TARGET}" 2>/dev/null || echo "0")
    if [ "$FILE_SIZE" -eq 0 ]; then
        echo "FATAL: Downloaded plugin file is empty"
        rm -f "${PLUGIN_TARGET}"
        exit 1
    fi
    
    echo "Plugin file downloaded: ${PLUGIN_TARGET} (size: ${FILE_SIZE} bytes)"
    
    # Ensure correct permissions
    chmod 644 "${PLUGIN_TARGET}" || true
    ls -la "${PLUGIN_TARGET}" || true
else
    echo "Plugin ${PLUGIN_FILE} already exists at ${PLUGIN_TARGET}"
    ls -la "${PLUGIN_TARGET}" || true
fi

# Verify the plugin file exists and is readable
echo "Verifying plugin file..."
if [ ! -f "${PLUGIN_TARGET}" ]; then
    echo "ERROR: Plugin file not found at ${PLUGIN_TARGET}"
    echo "Contents of ${ACTUAL_PLUGIN_DIR}:"
    ls -la "${ACTUAL_PLUGIN_DIR}" || true
    exit 1
fi

if [ ! -r "${PLUGIN_TARGET}" ]; then
    echo "ERROR: Plugin file exists but is not readable"
    ls -la "${PLUGIN_TARGET}" || true
    exit 1
fi

echo "Plugin file verified: ${PLUGIN_TARGET}"
echo "Listing all .ez files in plugin directories..."
find "${ACTUAL_PLUGIN_DIR}" -name "*.ez" -type f 2>/dev/null | head -20 || true

echo "Listing available plugins before enabling..."
rabbitmq-plugins list || true

# Enable the delayed message exchange plugin
echo "Enabling ${PLUGIN_NAME} plugin..."
if rabbitmq-plugins enable "${PLUGIN_NAME}"; then
    echo "Plugin enabled successfully"
else
    echo "Warning: Failed to enable ${PLUGIN_NAME} plugin"
    echo "Trying to enable with --offline flag..."
    rabbitmq-plugins enable --offline "${PLUGIN_NAME}" || true
fi

# Stop RabbitMQ gracefully
echo "Stopping RabbitMQ..."
rabbitmqctl stop

# Wait for process to fully stop
sleep 3

# Now start RabbitMQ in foreground (this will be the main process)
echo "Starting RabbitMQ in foreground..."
exec /usr/local/bin/docker-entrypoint.sh rabbitmq-server
