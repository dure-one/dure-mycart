# XMPP Integration Implementation

## Goal

Replace existing XMPP library stub with working `meszmate/xmpp-go` implementation for periodic message synchronization via MAM (XEP-0313) and real connection testing.

## Architecture

**Component Flow:**
```
CronRunner (existing)
    ↓ calls every 15min (configurable)
XMPPWorker (rewrite internals)
    ↓ uses
meszmate/xmpp-go client
    ↓ fetches from
XMPP Server (MAM XEP-0313)
    ↓ stores to
Database (existing tables)
```

**Tech Stack:**
- Go 1.21+
- `github.com/meszmate/xmpp-go` v1.x (replace `xmppo/go-xmpp`)
- Existing database schema (customer_contact, message tables)
- Existing cron runner infrastructure

## Global Constraints

- No new database tables - use existing responder schema
- Maintain existing cron job structure - worker must have `fetchMessages(ctx)` method
- Settings stored in database (responder_xmpp_* keys)
- Error handling: fail fast, log clearly, no internal retries
- MAM query proper XEP-0313 stanzas (not raw XML string concatenation)

---

## Component 1: Dependency Migration

**Files:**
- `go.mod`

**Changes:**
- Remove: `github.com/xmppo/go-xmpp`
- Add: `github.com/meszmate/xmpp-go v1.x.x` (check latest stable version)

**Verification:**
```bash
go mod tidy
go build ./...
```

---

## Component 2: Connection Management

**Files:**
- `internal/responder/xmpp_worker.go`

**XMPPWorker struct (updated):**
```go
type XMPPWorker struct {
    settings *models.ResponderSettings
    db       *queries.Base
    client   *xmpp.Client  // meszmate/xmpp-go client
    lastSync time.Time
}
```

**connect() method:**

Builds JID from settings (extract local part from full JID), creates client with TLS enabled, connects and authenticates.

```go
func (w *XMPPWorker) connect() error {
    jid := xmpp.JID{
        Local:  extractLocal(w.settings.XMPPJID),
        Domain: w.settings.XMPPServer,
    }
    
    client, err := xmpp.NewClient(
        jid,
        w.settings.XMPPPassword,
        xmpp.WithAddress(fmt.Sprintf("%s:%d", w.settings.XMPPServer, w.settings.XMPPPort)),
        xmpp.WithTLS(true),
    )
    if err != nil {
        return fmt.Errorf("new client: %w", err)
    }
    
    if err := client.Connect(); err != nil {
        return fmt.Errorf("connect: %w", err)
    }
    
    w.client = client
    return nil
}
```

**disconnect() method:**

Clean disconnect, safe to call multiple times.

```go
func (w *XMPPWorker) disconnect() {
    if w.client != nil {
        _ = w.client.Close()
    }
}
```

**Helper function:**

```go
func extractLocal(jid string) string {
    if idx := strings.Index(jid, "@"); idx > 0 {
        return jid[:idx]
    }
    return jid
}
```

**Error types:**
- Network errors: DNS failure, connection refused, timeout
- Auth errors: invalid credentials, SASL failure
- TLS errors: certificate validation failure

All errors wrapped with context (`fmt.Errorf("connect to %s:%d: %w", ...)`)

---

## Component 3: MAM Message Fetching

**Files:**
- `internal/responder/xmpp_worker.go`

**fetchMessages() method:**

Queries MAM for messages since lastSync, processes each message, updates lastSync on success.

```go
func (w *XMPPWorker) fetchMessages(ctx context.Context) error {
    if w.client == nil {
        return fmt.Errorf("not connected")
    }
    
    mamQuery := &mam.Query{
        Start: w.lastSync,
        End:   time.Now(),
    }
    
    results, err := w.client.QueryMAM(ctx, mamQuery)
    if err != nil {
        return fmt.Errorf("query MAM: %w", err)
    }
    
    for _, msg := range results.Messages {
        if err := w.processMessage(ctx, msg); err != nil {
            // ponytail: log but continue - one message failure shouldn't stop sync
            continue
        }
    }
    
    w.lastSync = time.Now()
    return nil
}
```

**processMessage() method:**

Extracts sender JID (bare, no resource), determines direction (inbound vs outbound by comparing sender to bot JID), creates database record.

```go
func (w *XMPPWorker) processMessage(ctx context.Context, msg *xmpp.Message) error {
    if msg.Body == "" {
        return nil
    }
    
    from := msg.From.Bare()
    
    contact, _, err := w.db.GetOrCreateContact(ctx, "xmpp", from)
    if err != nil {
        return fmt.Errorf("get or create contact: %w", err)
    }
    
    direction := "inbound"
    if msg.From.Bare() == w.settings.XMPPJID {
        direction = "outbound"
    }
    
    dbMsg := &models.Message{
        ContactID: contact.ID,
        Content:   msg.Body,
        Direction: direction,
        Channel:   "xmpp",
    }
    
    return w.db.CreateMessage(ctx, dbMsg)
}
```

**Message deduplication:**

Relies on lastSync timestamp - messages fetched once per time window. MAM returns chronological results, no ID-based tracking needed.

**Batch behavior:**

All messages in MAM result processed before updating lastSync. Partial failure (some messages fail to store) still advances lastSync - prevents infinite retry on broken message.

**API assumptions:**

The implementation assumes `meszmate/xmpp-go` provides:
- `client.QueryMAM(ctx, query)` returning message batch
- Messages with `.From` (JID type), `.Body` (string)
- JID with `.Bare()` method

If actual library API differs, adapt to real method names while preserving core logic.

---

## Component 4: Connection Test Endpoint

**Files:**
- `internal/handlers/private/responder.go`

**XMPPConnectionTest handler:**

Replace stub with real test. Creates temporary worker, connects, disconnects, returns success/failure to UI.

```go
func XMPPConnectionTest(c fiber.Ctx) error {
    db := queries.DB()
    log := logging.New()
    
    // Load current settings
    var settings models.ResponderSettings
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if _, err := db.GetSettingByGroup(ctx, &settings); err != nil {
        log.ErrorStack(err)
        return webutil.StatusInternalServerError(c)
    }
    
    // Validate settings exist
    if settings.XMPPJID == "" || settings.XMPPPassword == "" {
        return webutil.Response(c, fiber.StatusOK, "XMPP settings incomplete", map[string]any{
            "success": false,
            "error":   "XMPP JID and password required",
        })
    }
    
    // Test connection
    worker := responder.NewXMPPWorker(&settings, nil)
    if err := worker.Connect(); err != nil {
        return webutil.Response(c, fiber.StatusOK, "XMPP connection failed", map[string]any{
            "success": false,
            "error":   err.Error(),
        })
    }
    defer worker.Disconnect()
    
    return webutil.Response(c, fiber.StatusOK, "XMPP connection successful", map[string]any{
        "success": true,
    })
}
```

**Export Connect/Disconnect:**

Worker methods must be exported for handler access:

```go
// In xmpp_worker.go
func (w *XMPPWorker) Connect() error { return w.connect() }
func (w *XMPPWorker) Disconnect() { w.disconnect() }
```

**Timeout:**

5 second context timeout prevents hanging UI on unresponsive server.

**Error messages:**

- Settings incomplete: "XMPP JID and password required"
- Connection failure: Library error message (e.g., "dial tcp: connection refused")
- Auth failure: Library auth error (e.g., "SASL authentication failed")

Frontend already handles `response.success` boolean for green/red styling.

---

## Component 5: Error Handling

**Strategy:** Fail fast with clear errors. No retries inside worker - cron interval provides natural retry cadence.

**Connection errors:**

Network, DNS, TLS errors wrapped with context:
```go
return fmt.Errorf("connect to %s:%d: %w", server, port, err)
```

**Authentication errors:**

Check for typed auth error if library provides it:
```go
if errors.Is(err, xmpp.ErrAuthFailed) {
    return fmt.Errorf("authentication failed for %s: check credentials", jid)
}
```

**MAM query errors:**

MAM not supported, timeout, or permission denied:
```go
return fmt.Errorf("MAM query failed: %w", err)
```

**Message processing errors:**

Individual message failures logged but don't stop sync:
```go
for _, msg := range results.Messages {
    if err := w.processMessage(ctx, msg); err != nil {
        // ponytail: log error but continue
        continue
    }
}
```

**Cron runner behavior:**

Already handles job errors (from `cron_runner.go:58`):
```go
if err := r.executeJob(ctx, job.JobType); err != nil {
    // Log error but continue - doesn't stop other jobs
    continue
}
```

**Logging:**

- Worker errors: Logged in cron runner (existing behavior)
- Connection test: Error returned to UI (user feedback)
- Message processing: Individual errors logged, don't fail batch

**Visibility:**

- Settings page: Test button shows "Connection failed: <reason>" or "Connection successful!"
- Server logs: Cron failures appear with full error stack
- No database error state tracking (keep simple)

---

## Component 6: Testing

**Unit tests:**

Test message processing without network:

```go
func TestProcessMessage(t *testing.T) {
    db := setupTestDB(t)
    worker := &XMPPWorker{
        db: db,
        settings: &models.ResponderSettings{XMPPJID: "bot@example.com"},
    }
    
    msg := &xmpp.Message{
        From: xmpp.JID{Local: "user", Domain: "example.com"},
        Body: "test message",
    }
    
    err := worker.processMessage(context.Background(), msg)
    require.NoError(t, err)
    
    messages, _ := db.ListMessagesByCustomer(ctx, contactID)
    assert.Len(t, messages, 1)
    assert.Equal(t, "test message", messages[0].Content)
    assert.Equal(t, "inbound", messages[0].Direction)
}
```

**Integration test (optional):**

Real XMPP connection, skipped by default:

```go
func TestXMPPConnection(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    settings := &models.ResponderSettings{
        XMPPJID:      os.Getenv("TEST_XMPP_JID"),
        XMPPPassword: os.Getenv("TEST_XMPP_PASSWORD"),
        XMPPServer:   os.Getenv("TEST_XMPP_SERVER"),
        XMPPPort:     5222,
    }
    
    err := TestConnection(settings)
    assert.NoError(t, err)
}
```

**Handler test:**

Mock settings, test response format:

```go
func TestXMPPConnectionTestHandler(t *testing.T) {
    app := fiber.New()
    app.Post("/test", XMPPConnectionTest)
    
    req := httptest.NewRequest("POST", "/test", nil)
    resp, _ := app.Test(req)
    
    assert.Equal(t, 200, resp.StatusCode)
    // Parse JSON, verify success field
}
```

**Manual testing checklist:**

1. Blank XMPP settings → test button shows "settings incomplete"
2. Invalid credentials → test fails with "authentication failed"
3. Valid credentials → test succeeds (green background)
4. Enable xmpp_check cron job → messages sync after interval
5. Check database → new rows in message table with correct direction
6. Invalid server address → test fails with "connection refused"

**No mock XMPP server** - unit tests cover message processing, optional integration test validates real connection.

---

## Success Criteria

1. **Connection test works:** Settings page test button connects to real XMPP server, returns success/failure with clear error messages
2. **MAM sync works:** Cron job fetches messages from MAM, stores in database with correct contact linking
3. **Direction detection works:** Outbound messages (from bot JID) marked as "outbound", others as "inbound"
4. **Error visibility:** Connection failures show in UI (test button) and server logs (cron runs)
5. **No breaking changes:** Existing cron runner, database queries, settings storage work unchanged
6. **Tests pass:** Unit tests for message processing, handler tests for connection endpoint

---

## Migration Notes

**Breaking changes:**

- Go import path changes (`xmppo/go-xmpp` → `meszmate/xmpp-go`)
- Worker internal API changes (but cron runner interface stays same)
- No data migration needed - existing messages/contacts unaffected

**Deployment:**

1. Update `go.mod` dependency
2. Rebuild binary: `go build -o mycart cmd/main.go`
3. Restart server - existing cron jobs continue working
4. Test connection from settings page
5. Enable xmpp_check cron job if disabled

**Rollback:**

Revert `go.mod` changes, restore old `xmpp_worker.go`, rebuild. No database changes to undo.
