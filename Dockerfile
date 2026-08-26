# Multi-stage Dockerfile for distroless mycart

##
## Stage 1: Build frontend assets
##
FROM node:22-alpine AS frontend-builder

WORKDIR /app

# Copy package files for better caching
COPY web/admin/package*.json ./web/admin/
COPY web/site/package*.json ./web/site/

# Install dependencies
WORKDIR /app/web/admin
RUN npm ci

WORKDIR /app/web/site
RUN npm ci

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
    -ldflags="-w -s" \
    -o /go/bin/mycart \
    ./cmd/main.go

##
## Stage 3: Deploy into ultra-secure Distroless image
##
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

# Copy the binary with proper ownership
# The binary and its workdir are owned by nonroot so the app can create its
# runtime-writable directories (lc_base, lc_uploads, lc_digitals, lc_certs).
COPY --from=backend-builder --chown=nonroot:nonroot /go/bin/mycart /app/mycart

# Expose port
EXPOSE 8080

# Run as nonroot user (UID 65532) for enhanced security
USER nonroot:nonroot

ENTRYPOINT ["/app/mycart"]
CMD ["serve"]
