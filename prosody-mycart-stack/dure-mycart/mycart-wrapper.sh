#!/bin/busybox sh
# Wrapper to set environment variables for dure-mycart
# Horust v0.1.13 doesn't properly pass [environment] section to child processes

export MYCART_DOMAIN="${MYCART_DOMAIN}"
export MYCART_DEV_MODE="${MYCART_DEV_MODE}"
export MYCART_HTTP_ADDR="${MYCART_HTTP_ADDR}"
export MYCART_HTTPS_ADDR="${MYCART_HTTPS_ADDR}"
export GIN_MODE="${GIN_MODE:-release}"
export REVERSE_PROXY_BINDINGS="${REVERSE_PROXY_BINDINGS}"
export ACME_PROVIDER="${ACME_PROVIDER}"
export ACME_DIRECTORY_URL="${ACME_DIRECTORY_URL}"
export ACME_EMAIL="${ACME_EMAIL}"
export ACME_STAGING="${ACME_STAGING}"
export GTS_EAB_KID="${GTS_EAB_KID}"
export GTS_EAB_HMAC_KEY="${GTS_EAB_HMAC_KEY}"

exec /app/dure-mycart serve --http 0.0.0.0:80 --https 0.0.0.0:443
