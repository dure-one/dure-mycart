# Multi-stage Dockerfile for dure-mycart + Prosody + fail2ban-rs (distroless)

ARG FAIL2BAN_RS_VERSION=latest
ARG HORUST_VERSION=0.1.13

##
## Stage 1: frontend-builder (Bun)
##
FROM oven/bun:1-alpine AS frontend-builder

WORKDIR /app

# Copy scripts directory needed by postinstall
COPY scripts ./scripts

# Copy package files for better caching (bun needs package.json + bun.lockb)
COPY web/admin/package.json web/admin/bun.lockb ./web/admin/
COPY web/site/package.json web/site/bun.lockb ./web/site/

# Install dependencies
# Note: postinstall script checks platform and exits early on non-BSD systems
WORKDIR /app/web/admin
RUN bun install --frozen-lockfile

WORKDIR /app/web/site
RUN bun install --frozen-lockfile

# Copy source and build
WORKDIR /app
COPY web/admin ./web/admin
COPY web/site ./web/site

WORKDIR /app/web/admin
RUN bun run build

WORKDIR /app/web/site
RUN bun run build

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

# Install swag for swagger doc generation
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6

# Copy source code
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY docs ./docs
COPY migrations ./migrations
COPY web/embed.go ./web/embed.go

# Generate swagger docs (required by cmd/main.go import)
RUN /go/bin/swag init -g cmd/main.go --output docs/swagger --parseDependency --parseInternal

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/admin/build ./web/admin/build
COPY --from=frontend-builder /app/web/site/build ./web/site/build

# Build the binary with version info
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -tags sqlc \
    -ldflags="-w -s \
      -X 'main.version=${VERSION}' \
      -X 'main.gitCommit=${GIT_COMMIT}' \
      -X 'main.buildDate=${BUILD_DATE}'" \
    -o /go/bin/dure-mycart \
    ./cmd/main.go

##
## Stage 3: fail2ban-builder (Debian)
##
FROM debian:13-slim AS fail2ban-builder

ARG TARGETARCH
ARG FAIL2BAN_RS_VERSION

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Download fail2ban-rs
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        F2B_ARCH="amd64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        F2B_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $TARGETARCH" >&2; \
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
    find /tmp -type f -name "fail2ban-rs" -exec mv {} /usr/local/bin/fail2ban-rs \; && \
    if [ ! -f /usr/local/bin/fail2ban-rs ]; then \
        echo "ERROR: fail2ban-rs binary not found after extraction" >&2; \
        echo "Contents of /tmp:" >&2; \
        ls -la /tmp/ >&2; \
        exit 1; \
    fi && \
    chmod +x /usr/local/bin/fail2ban-rs && \
    rm -rf /tmp/fail2ban-rs.tar.gz /tmp/fail2ban-rs-*-linux-* && \
    /usr/local/bin/fail2ban-rs --version

##
## Stage 4: horust-builder (Alpine)
##
FROM alpine:latest AS horust-builder

ARG TARGETARCH
ARG HORUST_VERSION

RUN apk add --no-cache curl

WORKDIR /build

# Download Horust binary from GitHub releases
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        HORUST_ARCH="x86_64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        HORUST_ARCH="aarch64"; \
    else \
        echo "Unsupported architecture: $TARGETARCH" >&2; \
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
COPY prosody-mycart-stack/docker-entrypoint.sh /docker-entrypoint.sh
COPY prosody-mycart-stack/mycart-wrapper.sh /usr/local/bin/mycart-wrapper.sh
COPY prosody-mycart-stack/horust-services/ /etc/horust/services/

RUN mkdir -p /etc/fail2ban-rs && \
    chmod +x /docker-entrypoint.sh /usr/local/bin/mycart-wrapper.sh && \
    cp /bin/busybox /bin/busybox-static

COPY prosody-mycart-stack/fail2ban-rs-config.toml /etc/fail2ban-rs/config.toml

##
## Final Stage: distroless
##
FROM gcr.io/distroless/base-debian13:latest

ARG TARGETARCH

# Copy dure-mycart binary
COPY --from=backend-builder /go/bin/dure-mycart /app/dure-mycart

# Copy fail2ban-rs binary
COPY --from=fail2ban-builder /usr/local/bin/fail2ban-rs /usr/local/bin/fail2ban-rs

# Copy Horust
COPY --from=horust-builder /usr/local/bin/horust /usr/local/bin/horust

# Copy our tools and configs
COPY --from=tools-builder /docker-entrypoint.sh /docker-entrypoint.sh
COPY --from=tools-builder /usr/local/bin/mycart-wrapper.sh /usr/local/bin/mycart-wrapper.sh
COPY --from=tools-builder /etc/horust/services /etc/horust/services
COPY --from=tools-builder /etc/fail2ban-rs/config.toml /etc/fail2ban-rs/config.toml
COPY --from=tools-builder /bin/busybox-static /bin/busybox

# Copy openssl, curl, envsubst, nft
COPY --from=tools-builder /usr/bin/openssl /usr/bin/openssl
COPY --from=tools-builder /usr/bin/curl /usr/bin/curl
COPY --from=tools-builder /usr/bin/envsubst /usr/bin/envsubst
COPY --from=tools-builder /usr/sbin/nft /usr/sbin/nft

# Copy libraries - use architecture-specific paths
# amd64 -> x86_64-linux-gnu, arm64 -> aarch64-linux-gnu
COPY --from=tools-builder /usr/lib/*-linux-gnu /usr/lib/
COPY --from=tools-builder /etc/ssl/openssl.cnf /etc/ssl/openssl.cnf
COPY --from=tools-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=tools-builder /usr/lib/ssl /usr/lib/ssl

# Create necessary directories
RUN ["/bin/busybox", "mkdir", "-p", \
    "/certs", \
    "/logs", \
    "/etc/fail2ban-rs", \
    "/var/run/acme/acme-challenge", \
    "/var/lib/fail2ban-rs", \
    "/var/spool/cron/crontabs", \
    "/app/lc_base", \
    "/app/lc_uploads", \
    "/app/lc_digitals", \
    "/run"]

# Create root crontab file with proper permissions
RUN ["/bin/busybox", "touch", "/var/spool/cron/crontabs/root"]
RUN ["/bin/busybox", "chmod", "0600", "/var/spool/cron/crontabs/root"]

# Create utility symlinks for tools (no shell symlinks for security)
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/grep"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/awk"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/md5sum"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/mktemp"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/mv"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/rm"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/cat"]
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/crontab"]

# Expose ports (informational only with host networking)
EXPOSE 80 443

# Entrypoint
ENTRYPOINT ["/docker-entrypoint.sh"]
