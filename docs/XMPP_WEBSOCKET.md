# XMPP WebSocket Support

This document describes XMPP over WebSocket (RFC 7395) support in dure-mycart with XEP-0156 metadata advertisement.

## Overview

dure-mycart advertises XMPP WebSocket endpoints via the `.well-known/host-meta` discovery mechanism defined in XEP-0156. This allows XMPP clients (especially browser-based clients like Converse.js) to automatically discover and connect to the WebSocket service.

## How It Works

### 1. Discovery Endpoints

Two public endpoints are available for XMPP WebSocket discovery:

**XML Format** (XRD):
```
GET /.well-known/host-meta
Content-Type: application/xrd+xml
Access-Control-Allow-Origin: *
```

**JSON Format** (JRD):
```
GET /.well-known/host-meta.json
Content-Type: application/jrd+json
Access-Control-Allow-Origin: *
```

### 2. Probe Mechanism

Instead of trusting database configuration, the endpoints **actively probe** the WebSocket service before advertising it:

- Attempts WebSocket upgrade with `Sec-WebSocket-Protocol: xmpp` header
- Verifies the server negotiates the `xmpp` subprotocol
- Returns 500ms timeout if unreachable
- **Caches results for 60 seconds** to avoid hammering the WebSocket service

### 3. Response Formats

**When WebSocket is available:**

XML:
```xml
<?xml version='1.0' encoding='utf-8'?>
<XRD xmlns='http://docs.oasis-open.org/ns/xri/xrd-1.0'>
  <Link rel="urn:xmpp:alt-connections:websocket" href="wss://example.com:5280/xmpp-websocket" />
</XRD>
```

JSON:
```json
{
  "links": [
    {
      "rel": "urn:xmpp:alt-connections:websocket",
      "href": "wss://example.com:5280/xmpp-websocket"
    }
  ]
}
```

**When WebSocket is unavailable:**

XML:
```xml
<?xml version='1.0' encoding='utf-8'?>
<XRD xmlns='http://docs.oasis-open.org/ns/xri/xrd-1.0'>
</XRD>
```

JSON:
```json
{
  "links": []
}
```

## Configuration

### Environment Variable

Set the WebSocket URL via environment variable:

```bash
export XMPP_WEBSOCKET_URL="wss://your-domain.com:5280/xmpp-websocket"
```

**Default:** `wss://localhost:5280/xmpp-websocket` (Prosody default)

### XMPP Server Setup (Prosody)

Enable the WebSocket module in `prosody.cfg.lua`:

```lua
-- Enable HTTP modules
modules_enabled = {
    -- ... other modules
    "websocket";
}

-- WebSocket configuration
consider_websocket_secure = true;

-- Optional: Enable built-in host-meta serving
modules_enabled = {
    "http_altconnect";
}
```

**Note:** If you enable Prosody's `http_altconnect` module, it will serve its own `.well-known/host-meta`. dure-mycart's endpoints are designed to work alongside or replace it.

### Verify XMPP Subprotocol Support

**CRITICAL:** Your WebSocket handler MUST support the `xmpp` subprotocol per RFC 7395.

If using **GoFiber WebSocket** (for custom proxy):

```go
import "github.com/gofiber/contrib/v3/websocket"

cfg := websocket.Config{
    Subprotocols: []string{"xmpp"},  // REQUIRED
}

app.Get("/xmpp-websocket", websocket.New(func(c *websocket.Conn) {
    defer c.Close()
    
    // Log negotiated subprotocol
    log.Printf("XMPP WS session. Subprotocol: %s", c.Subprotocol())
    
    // Handle XMPP XML stanzas
    for {
        mt, msg, err := c.ReadMessage()
        if err != nil {
            log.Printf("XMPP WS closed: %v", err)
            break
        }
        // Forward to XMPP server or handle locally
        // ...
    }
}, cfg))
```

## Testing

### Manual Test

```bash
# Test XML endpoint
curl -v http://localhost:8080/.well-known/host-meta

# Test JSON endpoint
curl -s http://localhost:8080/.well-known/host-meta.json | jq .

# Verify CORS headers
curl -sI http://localhost:8080/.well-known/host-meta.json | grep -i access-control
```

### Test with XMPP Client

**Converse.js** (browser-based):

```html
<script src="https://cdn.conversejs.org/dist/converse.min.js"></script>
<script>
converse.initialize({
    websocket_url: 'auto',  // Uses .well-known/host-meta
    jid: 'user@your-domain.com',
    // ... other config
});
</script>
```

**Strophe.js**:

```javascript
const conn = new Strophe.Connection('wss://your-domain.com:5280/xmpp-websocket');
conn.connect('user@your-domain.com', 'password', onConnect);
```

### Automated Tests

```bash
# Run unit tests
go test ./internal/handlers/public -run TestWellKnown -v

# Run all tests
go test ./internal/handlers/public -count=1
```

## Architecture

### dure-mycart Stack

```
┌──────────────────────────────────────────────────┐
│ Browser-based XMPP Client (Converse.js)         │
└──────────────────────────────────────────────────┘
                    │
                    │ 1. GET /.well-known/host-meta
                    ↓
┌──────────────────────────────────────────────────┐
│ dure-mycart (Go/Fiber)                           │
│  - Probes WebSocket (500ms timeout)              │
│  - Caches result (60s TTL)                       │
│  - Returns metadata if available                 │
└──────────────────────────────────────────────────┘
                    │
                    │ 2. Probe: ws:// upgrade
                    ↓
┌──────────────────────────────────────────────────┐
│ xmpp-proxy (QUIC/WebSocket)                      │
│  - Accepts WebSocket with xmpp subprotocol       │
│  - Forwards to Prosody                           │
└──────────────────────────────────────────────────┘
                    │
                    │ 3. XMPP traffic
                    ↓
┌──────────────────────────────────────────────────┐
│ Prosody XMPP Server                              │
└──────────────────────────────────────────────────┘
```

### Three-Container Stack (prosody-mycart-stack/)

- **dure-mycart**: HTTP (80/tcp), serves well-known endpoints
- **xmpp-proxy**: XMPP protocols (443/udp QUIC, 5222/5223/5269 Direct TLS)
- **prosody**: XMPP backend (172.19.0.2)

## Troubleshooting

### Probe Always Fails

**Symptom:** `.well-known/host-meta` always returns empty metadata.

**Causes:**
1. WebSocket service not running
2. Firewall blocking connection
3. Wrong `XMPP_WEBSOCKET_URL` (check protocol: `ws://` vs `wss://`)
4. Subprotocol not configured (server rejects `xmpp` subprotocol)

**Debug:**
```bash
# Check if WebSocket responds
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: $(echo -n "$RANDOM" | base64)" \
  -H "Sec-WebSocket-Protocol: xmpp" \
  http://localhost:5280/xmpp-websocket

# Should return:
# HTTP/1.1 101 Switching Protocols
# Sec-WebSocket-Protocol: xmpp
```

### Cache Stale

**Symptom:** WebSocket is now available but metadata still shows empty.

**Solution:** Wait 60 seconds for cache to expire, or restart dure-mycart.

### CORS Errors

**Symptom:** Browser blocks `.well-known/host-meta` request.

**Check:** Response must include `Access-Control-Allow-Origin: *`

```bash
curl -sI http://localhost:8080/.well-known/host-meta.json | grep -i access-control
# Should output: Access-Control-Allow-Origin: *
```

## References

- **RFC 7395**: XMPP over WebSocket
  - https://datatracker.ietf.org/doc/html/rfc7395
- **XEP-0156**: Discovering Alternative XMPP Connection Methods
  - https://xmpp.org/extensions/xep-0156.html
- **XEP-0467**: XMPP over QUIC
  - https://xmpp.org/extensions/xep-0467.html
- **Prosody WebSocket Module**:
  - https://prosody.im/doc/websocket
- **Converse.js Documentation**:
  - https://conversejs.org/docs/html/

## Security Notes

- **CORS is wide-open** (`Access-Control-Allow-Origin: *`) on well-known endpoints only. This is required by XEP-0156 for browser-based clients.
- **No authentication** required for discovery endpoints (by design).
- **TLS strongly recommended** for production (`wss://` not `ws://`).
- **Probe timeout** is 500ms to prevent DoS via slow WebSocket responses.
