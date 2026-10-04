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
)

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

// connect establishes XMPP connection
func (w *XMPPWorker) connect() error {
	// Extract domain from JID if XMPPServer not provided
	server := w.settings.XMPPServer
	if server == "" {
		server = extractDomain(w.settings.XMPPJID)
	}

	// Default to port 443 (XMPP-over-TLS) if not specified
	port := w.settings.XMPPPort
	if port == 0 {
		port = 443
	}

	// Build JID from settings
	userJID, err := jid.New(extractLocal(w.settings.XMPPJID), server, "")
	if err != nil {
		return fmt.Errorf("parse JID: %w", err)
	}

	// Determine connection address: use XMPPConnectAddr if set, otherwise server
	connectAddr := server
	if w.settings.XMPPConnectAddr != "" {
		connectAddr = w.settings.XMPPConnectAddr
	}

	// Configure ALPN for direct TLS on port 443 (XEP-0368), STARTTLS for other ports
	var options []xmpp.ClientOption
	if port == 443 {
		tlsConfig := &tls.Config{
			ServerName: server,
			NextProtos: []string{"xmpp-client"}, // ALPN protocol for C2S
		}
		options = append(options,
			xmpp.WithDirectTLS(),
			xmpp.WithClientTLS(tlsConfig),
		)
	}
	options = append(options, xmpp.WithConnectAddr(fmt.Sprintf("%s:%d", connectAddr, port)))

	// Create client
	client, err := xmpp.NewClient(
		userJID,
		w.settings.XMPPPassword,
		options...,
	)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	// Connect and authenticate
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect to %s:%d: %w", connectAddr, port, err)
	}

	w.client = client
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
	if w.client == nil {
		return fmt.Errorf("not connected")
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
