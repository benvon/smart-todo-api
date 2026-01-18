#!/bin/sh
# Generate config.json from environment variable at runtime

set -e

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

# Generate config.json in /tmp (writable location)
cat > /tmp/config.json <<EOF
{
  "api_base_url": "${API_BASE_URL}"
}
EOF

echo "Generated config.json with api_base_url=${API_BASE_URL}"

# Start nginx in the foreground (default config includes /etc/nginx/conf.d/*.conf)
exec nginx -g 'daemon off;'
