# Build stage: Build the frontend with Node.js
FROM node:18-alpine AS builder

WORKDIR /build

# Copy package files
COPY web/package.json web/package-lock.json* ./

# Install dependencies (including devDependencies for build)
RUN npm install

# Copy web source files
COPY web/ ./

# Build the frontend
RUN npm run build

# Production stage: Serve with nginx
FROM nginxinc/nginx-unprivileged:1-trixie-otel

# Copy built files from builder stage
COPY --from=builder /build/dist /usr/share/nginx/html/dist
COPY web/index.html /usr/share/nginx/html/
COPY web/app.html /usr/share/nginx/html/
COPY web/css/ /usr/share/nginx/html/css/
COPY web/manifest.json /usr/share/nginx/html/
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY scripts/generate_config.sh /usr/local/bin/generate_config.sh

EXPOSE 80

CMD ["sh", "/usr/local/bin/generate_config.sh"]
