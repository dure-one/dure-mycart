#!/bin/busybox sh
# Wrapper to set environment variables for mycart
# Horust v0.1.13 doesn't properly pass [environment] section to child processes

export MYCART_DOMAIN="${MYCART_DOMAIN}"
export GIN_MODE="${GIN_MODE:-release}"
export REVERSE_PROXY_BINDINGS="${REVERSE_PROXY_BINDINGS}"

exec /app/mycart serve --http 0.0.0.0:80 --https 0.0.0.0:443
