# Multi-stage Dockerfile for distroless dure-mycart

##
## Stage 1: Build frontend assets
##
FROM node:22-alpine AS frontend-builder

WORKDIR /app

# Copy scripts directory needed by postinstall
COPY scripts ./scripts

# Copy package files for better caching
COPY web/admin/package*.json ./web/admin/
COPY web/site/package*.json ./web/site/

# Install dependencies
# Note: postinstall script checks platform and exits early on non-BSD systems
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
## Stage 2: Build Go binary
##
FROM golang:1.26-alpine AS backend-builder

WORKDIR /go/src/app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/admin/build ./web/admin/build
COPY --from=frontend-builder /app/web/site/build ./web/site/build

# Build the binary
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -tags sqlc \
    -ldflags="-w -s" \
    -o /go/bin/dure-mycart \
    ./cmd/main.go

##
## Stage 3: Tools builder (for busybox and crontab support)
##
FROM debian:13-slim AS tools-builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends busybox-static && \
    rm -rf /var/lib/apt/lists/*

##
## Stage 4: Deploy into ultra-secure Distroless image
##
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

# Copy the binary with proper ownership
# The binary and its workdir are owned by nonroot so the app can create its
# runtime-writable directories (lc_base, lc_uploads, lc_digitals, lc_certs).
COPY --from=backend-builder --chown=nonroot:nonroot /go/bin/dure-mycart /app/dure-mycart

# Copy busybox for crontab support
COPY --from=tools-builder /bin/busybox /bin/busybox

# Create crontab directories with proper ownership for nonroot user (UID 65532)
RUN ["/bin/busybox", "mkdir", "-p", "/var/spool/cron/crontabs"]
RUN ["/bin/busybox", "chown", "-R", "65532:65532", "/var/spool/cron/crontabs"]

# Create crontab symlink
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/usr/bin/crontab"]

# Expose port
EXPOSE 8080

# Run as nonroot user (UID 65532) for enhanced security
USER nonroot:nonroot

ENTRYPOINT ["/app/dure-mycart"]
CMD ["serve"]
