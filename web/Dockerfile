# Kootenai Platform Web UI
# Multi-stage build for Vue.js frontend

# Build stage
FROM node:20-alpine AS builder

WORKDIR /build

# Build arguments for Vite
ARG VITE_USE_MOCK_DATA=false
ENV VITE_USE_MOCK_DATA=${VITE_USE_MOCK_DATA}

# Copy package files first for better caching
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Runtime stage - nginx to serve static files
FROM nginx:1.27-alpine3.21 AS runtime

# Patch OS-level vulnerabilities and install envsubst
RUN apk upgrade --no-cache && apk add --no-cache gettext

# Copy custom nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy built files from builder
COPY --from=builder /build/dist /usr/share/nginx/html

# Create config for runtime environment variables
RUN echo 'window.APP_CONFIG = { apiUrl: "${API_URL}" };' > /usr/share/nginx/html/config.js.template

# Script to replace env vars at runtime
COPY docker-entrypoint.sh /docker-entrypoint.sh

# Set up permissions for non-root nginx user
RUN mkdir -p /var/cache/nginx /var/run /var/log/nginx && \
    chown -R nginx:nginx /var/cache/nginx /var/run /var/log/nginx /usr/share/nginx/html && \
    chmod +x /docker-entrypoint.sh && \
    touch /var/run/nginx.pid && chown nginx:nginx /var/run/nginx.pid

USER nginx

EXPOSE 8080 8443

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
