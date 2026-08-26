# Multi-stage Dockerfile for mycart + xmpp-proxy + fail2ban-rs (distroless)

ARG XMPP_PROXY_VERSION=latest
ARG FAIL2BAN_RS_VERSION=latest
ARG HORUST_VERSION=0.1.13

##
## Stage 1: frontend-builder (Node.js)
##
FROM node:22-alpine AS frontend-builder

WORKDIR /app

# Copy package files for better caching
COPY web/admin/package*.json ./web/admin/
COPY web/site/package*.json ./web/site/

# Install dependencies
WORKDIR /app/web/admin
RUN npm ci --legacy-peer-deps

WORKDIR /app/web/site
RUN npm ci --legacy-peer-deps

# Copy source and build
WORKDIR /app
COPY web/admin ./web/admin
COPY web/site ./web/site

WORKDIR /app/web/admin
RUN npx vite build

WORKDIR /app/web/site
RUN npx vite build

##
## Stage 2: backend-builder (Go)
##
FROM golang:1.26-alpine AS backend-builder

WORKDIR /go/src/app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY docs ./docs
COPY migrations ./migrations
COPY web/embed.go ./web/embed.go

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/admin/build ./web/admin/build
COPY --from=frontend-builder /app/web/site/build ./web/site/build

# Build the binary
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s" \
    -o /go/bin/mycart \
    ./cmd/main.go

##
## Stage 3: xmpp-binaries-builder (Debian)
##
FROM debian:13-slim AS xmpp-binaries-builder

ARG XMPP_PROXY_VERSION
ARG FAIL2BAN_RS_VERSION

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Download xmpp-proxy
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
        XMPP_ARCH="x86_64"; \
    elif [ "$ARCH" = "aarch64" ]; then \
        XMPP_ARCH="aarch64"; \
    else \
        echo "Unsupported architecture: $ARCH" >&2; \
        exit 1; \
    fi && \
    if [ "$XMPP_PROXY_VERSION" = "latest" ]; then \
        DOWNLOAD_URL="https://github.com/nikescar/xmpp-proxy/releases/latest/download/xmpp-proxy-${XMPP_ARCH}-unknown-linux-musl"; \
    else \
        DOWNLOAD_URL="https://github.com/nikescar/xmpp-proxy/releases/download/${XMPP_PROXY_VERSION}/xmpp-proxy-${XMPP_ARCH}-unknown-linux-musl"; \
    fi && \
    echo "Downloading xmpp-proxy from ${DOWNLOAD_URL}..." && \
    curl -fsSL "$DOWNLOAD_URL" -o /usr/local/bin/xmpp-proxy && \
    chmod +x /usr/local/bin/xmpp-proxy

# Download fail2ban-rs
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
        F2B_ARCH="amd64"; \
    elif [ "$ARCH" = "aarch64" ]; then \
        F2B_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $ARCH" >&2; \
        exit 1; \
    fi && \
    if [ "$FAIL2BAN_RS_VERSION" = "latest" ]; then \
        TAG=$(curl -fsSL https://api.github.com/repos/aejimmi/fail2ban-rs/releases/latest | \
            grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'); \
    else \
        TAG="$FAIL2BAN_RS_VERSION"; \
    fi && \
    VER="${TAG#v}" && \
    DOWNLOAD_URL="https://github.com/aejimmi/fail2ban-rs/releases/download/${TAG}/fail2ban-rs-${VER}-linux-${F2B_ARCH}.tar.gz" && \
    echo "Downloading fail2ban-rs from ${DOWNLOAD_URL}..." && \
    curl -fsSL "$DOWNLOAD_URL" -o /tmp/fail2ban-rs.tar.gz && \
    tar -xzf /tmp/fail2ban-rs.tar.gz -C /tmp && \
    find /tmp -type f -name fail2ban-rs -exec mv {} /usr/local/bin/fail2ban-rs \; && \
    rm -rf /tmp/fail2ban-rs.tar.gz /tmp/fail2ban-rs-*-linux-* && \
    chmod +x /usr/local/bin/fail2ban-rs

##
## Stage 4: horust-builder (Alpine)
##
FROM alpine:latest AS horust-builder

ARG HORUST_VERSION

RUN apk add --no-cache curl

WORKDIR /build

# Download Horust binary from GitHub releases
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then \
        HORUST_ARCH="aarch64"; \
    elif [ "$ARCH" = "x86_64" ]; then \
        HORUST_ARCH="x86_64"; \
    else \
        echo "Unsupported architecture: $ARCH" >&2; \
        exit 1; \
    fi && \
    echo "Downloading Horust v${HORUST_VERSION} for ${HORUST_ARCH}..." && \
    curl -fsSL "https://github.com/FedericoPonzi/Horust/releases/download/v${HORUST_VERSION}/horust-${HORUST_ARCH}-unknown-linux-gnu.tar.gz" \
        -o /build/horust.tar.gz && \
    tar -xzf /build/horust.tar.gz -C /build && \
    mv /build/horust /usr/local/bin/horust && \
    chmod +x /usr/local/bin/horust

##
## Stage 5: tools-builder (Debian)
##
FROM debian:13-slim AS tools-builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        busybox-static \
        gettext-base \
        openssl \
        curl \
        ca-certificates \
        nftables && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy our scripts and configs
COPY xmpp-proxy-stack/docker-entrypoint.sh /docker-entrypoint.sh
COPY xmpp-proxy-stack/horust-services/ /etc/horust/services/

RUN chmod +x /docker-entrypoint.sh && \
    cp /bin/busybox /bin/busybox-static

##
## Final Stage: distroless
##
FROM gcr.io/distroless/base-debian13:latest

# Copy mycart binary
COPY --from=backend-builder /go/bin/mycart /app/mycart

# Copy xmpp binaries
COPY --from=xmpp-binaries-builder /usr/local/bin/xmpp-proxy /usr/local/bin/xmpp-proxy
COPY --from=xmpp-binaries-builder /usr/local/bin/fail2ban-rs /usr/local/bin/fail2ban-rs

# Copy Horust
COPY --from=horust-builder /usr/local/bin/horust /usr/local/bin/horust

# Copy our tools and configs
COPY --from=tools-builder /docker-entrypoint.sh /docker-entrypoint.sh
COPY --from=tools-builder /etc/horust/services /etc/horust/services
COPY --from=tools-builder /bin/busybox-static /bin/busybox

# Copy openssl, curl, envsubst, nft
COPY --from=tools-builder /usr/bin/openssl /usr/bin/openssl
COPY --from=tools-builder /usr/bin/curl /usr/bin/curl
COPY --from=tools-builder /usr/bin/envsubst /usr/bin/envsubst
COPY --from=tools-builder /usr/sbin/nft /usr/sbin/nft

# Copy libraries (full glob safe - tools-builder is same Debian release as this base)
COPY --from=tools-builder /usr/lib/x86_64-linux-gnu/*.so* /usr/lib/x86_64-linux-gnu/
COPY --from=tools-builder /etc/ssl/openssl.cnf /etc/ssl/openssl.cnf
COPY --from=tools-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=tools-builder /usr/lib/ssl /usr/lib/ssl

# Create necessary directories
RUN ["/bin/busybox", "mkdir", "-p", \
    "/certs", \
    "/logs", \
    "/etc/xmpp-proxy", \
    "/etc/fail2ban-rs", \
    "/var/run/acme/acme-challenge", \
    "/var/lib/fail2ban-rs", \
    "/app/lc_base", \
    "/app/lc_uploads", \
    "/app/lc_digitals", \
    "/run"]

# Create utility symlinks for tools (no shell symlinks for security)
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/grep"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/awk"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/md5sum"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/mktemp"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/mv"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/rm"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/cat"]

# Expose ports (informational only with host networking)
EXPOSE 80 443 5222 5223 5269

# Entrypoint
ENTRYPOINT ["/docker-entrypoint.sh"]
