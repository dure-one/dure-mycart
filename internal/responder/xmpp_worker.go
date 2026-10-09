package responder

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	xmpp "github.com/meszmate/xmpp-go"
	"github.com/meszmate/xmpp-go/jid"

	"github.com/dure-one/dure-mycart/internal/models"
	"github.com/dure-one/dure-mycart/internal/queries"
	"github.com/dure-one/dure-mycart/pkg/security"
)

// ConnectionAttempt tracks a single connection attempt
type ConnectionAttempt struct {
	Mode    string
	Address string
	Error   error
}

// XMPPWorker handles XMPP message synchronization via MAM
type XMPPWorker struct {
	settings *models.ResponderSettings
	db       *queries.Base
	client   *xmpp.Client
	lastSync time.Time
}

// NewXMPPWorker creates a new XMPP worker
func NewXMPPWorker(settings *models.ResponderSettings, db *queries.Base) *XMPPWorker {
	return &XMPPWorker{
		settings: settings,
		db:       db,
		lastSync: time.Now().Add(-24 * time.Hour),
	}
}

// Start begins the XMPP worker loop
func (w *XMPPWorker) Start(ctx context.Context) error {
	if err := w.connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer w.disconnect()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	if err := w.fetchMessages(ctx); err != nil {
		return fmt.Errorf("initial fetch: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.fetchMessages(ctx); err != nil {
				return fmt.Errorf("fetch messages: %w", err)
			}
		}
	}
}

// getDecryptedPassword decrypts the stored password
func (w *XMPPWorker) getDecryptedPassword() (string, error) {
	if w.settings.XMPPPassword == "" {
		return "", nil
	}

	// Try to decrypt - if it fails, assume it's already plaintext (for backward compatibility)
	decrypted, err := security.DecryptPassword(w.settings.XMPPPassword, security.GetEncryptionKey())
	if err != nil {
		// If decryption fails, try using the password as-is (might be plaintext)
		return w.settings.XMPPPassword, nil
	}

	return decrypted, nil
}

// extractLocal extracts the local part from a full JID (user@domain → user)
func extractLocal(jid string) string {
	if idx := strings.Index(jid, "@"); idx > 0 {
		return jid[:idx]
	}
	return jid
}

// extractDomain extracts domain from full JID (user@domain → domain)
func extractDomain(jid string) string {
	if idx := strings.Index(jid, "@"); idx > 0 && idx < len(jid)-1 {
		return jid[idx+1:]
	}
	return jid
}

// ConnectWithAllMethods tries all connection methods in order and returns attempts
func (w *XMPPWorker) ConnectWithAllMethods() (*xmpp.Client, []ConnectionAttempt) {
	server := w.settings.XMPPServer
	if server == "" {
		server = extractDomain(w.settings.XMPPJID)
	}

	attempts := []ConnectionAttempt{}

	// Auto-construct WebSocket and BOSH URLs from domain if not provided
	websocketURL := w.settings.XMPPWebSocketURL
	if websocketURL == "" {
		websocketURL = fmt.Sprintf("wss://%s/ws", server)
	}

	boshURL := w.settings.XMPPBOSHURL
	if boshURL == "" {
		boshURL = fmt.Sprintf("https://%s/http-bind/", server)
	}

	// Try in order: websocket → bosh → direct-tls → starttls
	modes := []struct {
		name string
		url  string
	}{
		{"websocket", websocketURL},
		{"bosh", boshURL},
		{"direct-tls", ""},
		{"starttls", ""},
	}

	for _, m := range modes {
		var client *xmpp.Client
		var addr string
		var err error

		if m.url != "" {
			client, addr, err = w.tryConnectWithURL(m.name, server, m.url)
		} else {
			client, addr, err = w.tryConnect(m.name, server)
		}

		attempts = append(attempts, ConnectionAttempt{
			Mode:    m.name,
			Address: addr,
			Error:   err,
		})

		if err == nil {
			w.client = client
			return client, attempts
		}
	}

	return nil, attempts
}

// tryConnectWithURL attempts connection with WebSocket or BOSH using provided URL
func (w *XMPPWorker) tryConnectWithURL(mode, server, url string) (*xmpp.Client, string, error) {
	userJID, err := jid.New(extractLocal(w.settings.XMPPJID), server, "")
	if err != nil {
		return nil, "", fmt.Errorf("parse JID: %w", err)
	}

	var options []xmpp.ClientOption

	switch mode {
	case "websocket":
		options = append(options, xmpp.WithWebSocket(url))
	case "bosh":
		options = append(options, xmpp.WithBOSH(url))
	default:
		return nil, "", fmt.Errorf("unsupported URL mode: %s", mode)
	}

	// Decrypt password before using it
	password, err := w.getDecryptedPassword()
	if err != nil {
		return nil, url, fmt.Errorf("decrypt password: %w", err)
	}

	client, err := xmpp.NewClient(userJID, password, options...)
	if err != nil {
		return nil, url, fmt.Errorf("create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return nil, url, err
	}

	return client, url, nil
}

// tryConnect attempts connection with specific mode
func (w *XMPPWorker) tryConnect(mode, server string) (*xmpp.Client, string, error) {
	port := w.settings.XMPPPort
	if port == 0 {
		if mode == "direct-tls" {
			port = 443
		} else if mode == "starttls" {
			port = 5222
		}
	}

	userJID, err := jid.New(extractLocal(w.settings.XMPPJID), server, "")
	if err != nil {
		return nil, "", fmt.Errorf("parse JID: %w", err)
	}

	var options []xmpp.ClientOption
	var addr string

	switch mode {
	case "direct-tls":
		addr = fmt.Sprintf("%s:%d", server, port)
		tlsConfig := &tls.Config{
			ServerName: server,
			NextProtos: []string{"xmpp-client"},
		}
		options = append(options,
			xmpp.WithDirectTLS(),
			xmpp.WithClientTLS(tlsConfig),
			xmpp.WithConnectAddr(addr),
		)

	case "starttls":
		addr = fmt.Sprintf("%s:%d", server, port)
		options = append(options, xmpp.WithConnectAddr(addr))

	default:
		return nil, "", fmt.Errorf("unsupported mode: %s", mode)
	}

	// Decrypt password before using it
	password, err := w.getDecryptedPassword()
	if err != nil {
		return nil, addr, fmt.Errorf("decrypt password: %w", err)
	}

	client, err := xmpp.NewClient(userJID, password, options...)
	if err != nil {
		return nil, addr, fmt.Errorf("create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return nil, addr, err
	}

	return client, addr, nil
}

// connect establishes XMPP connection (uses first successful method)
func (w *XMPPWorker) connect() error {
	client, attempts := w.ConnectWithAllMethods()
	if client == nil {
		var errMsg strings.Builder
		errMsg.WriteString("all connection methods failed:")
		for _, a := range attempts {
			errMsg.WriteString(fmt.Sprintf("\n  %s (%s): %v", a.Mode, a.Address, a.Error))
		}
		return fmt.Errorf("%s", errMsg.String())
	}
	return nil
}

// disconnect closes the XMPP connection
func (w *XMPPWorker) disconnect() {
	if w.client != nil {
		_ = w.client.Close()
	}
}

// Connect is exported wrapper for connection test
func (w *XMPPWorker) Connect() error {
	return w.connect()
}

// Disconnect is exported wrapper for connection test
func (w *XMPPWorker) Disconnect() {
	w.disconnect()
}

// fetchMessages queries MAM and syncs messages to database
// ponytail: MAM stub returns immediately until real server available for testing
func (w *XMPPWorker) fetchMessages(ctx context.Context) error {
	// Auto-connect if not connected (for cron-run mode)
	if w.client == nil {
		if err := w.connect(); err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer w.disconnect()
	}

	// ponytail: meszmate/xmpp-go uses plugin+handler pattern for message processing
	// requires Serve() with handler registration, not simple Send/Recv
	// stub returns success until we have MAM server to test against
	w.lastSync = time.Now()
	return nil
}

// ponytail: processMessage reserved for real MAM implementation
// will process forwarded messages from MAM query results
// requires handler pattern with stanza.Message type from meszmate/xmpp-go
