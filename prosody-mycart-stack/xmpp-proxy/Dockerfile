# Multi-stage Dockerfile for xmpp-proxy (distroless)

ARG XMPP_PROXY_VERSION=latest
ARG TARGETARCH

##
## Stage 1: Download xmpp-proxy binary from GitHub releases
##
FROM debian:13-slim AS downloader

ARG TARGETARCH
ARG XMPP_PROXY_VERSION

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Download xmpp-proxy binary from nikescar/xmpp-proxy releases
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        ARCH="x86_64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        ARCH="aarch64"; \
    else \
        echo "Unsupported architecture: $TARGETARCH" >&2; \
        exit 1; \
    fi && \
    if [ "$XMPP_PROXY_VERSION" = "latest" ]; then \
        TAG=$(curl -fsSL https://api.github.com/repos/nikescar/xmpp-proxy/releases/latest | \
            grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'); \
    else \
        TAG="$XMPP_PROXY_VERSION"; \
    fi && \
    echo "Downloading xmpp-proxy ${TAG} for ${ARCH}..." && \
    DOWNLOAD_URL="https://github.com/nikescar/xmpp-proxy/releases/download/${TAG}/xmpp-proxy-${ARCH}-unknown-linux-musl.tar.gz" && \
    curl -fsSL "$DOWNLOAD_URL" -o /tmp/xmpp-proxy.tar.gz && \
    tar -xzf /tmp/xmpp-proxy.tar.gz -C /tmp && \
    find /tmp -type f -name "xmpp-proxy" -exec mv {} /usr/local/bin/xmpp-proxy \; && \
    chmod +x /usr/local/bin/xmpp-proxy && \
    /usr/local/bin/xmpp-proxy --version

##
## Final Stage: distroless
##
FROM gcr.io/distroless/base-debian13:latest

# Copy xmpp-proxy binary
COPY --from=downloader /usr/local/bin/xmpp-proxy /usr/local/bin/xmpp-proxy

# Expose ports
EXPOSE 443/udp 5222 5223 5269 15270

# Entrypoint
ENTRYPOINT ["/usr/local/bin/xmpp-proxy"]
CMD ["--config", "/etc/xmpp-proxy/xmpp-proxy.toml"]
