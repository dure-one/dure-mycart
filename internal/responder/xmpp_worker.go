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

// connect establishes XMPP connection
func (w *XMPPWorker) connect() error {
	// Build JID from settings
	userJID, err := jid.New(extractLocal(w.settings.XMPPJID), w.settings.XMPPServer, "")
	if err != nil {
		return fmt.Errorf("parse JID: %w", err)
	}

	// Determine connection address: use XMPPConnectAddr if set, otherwise XMPPServer
	connectAddr := w.settings.XMPPServer
	if w.settings.XMPPConnectAddr != "" {
		connectAddr = w.settings.XMPPConnectAddr
	}

	// Configure ALPN for direct TLS on port 443 (XEP-0368)
	var options []xmpp.ClientOption
	if w.settings.XMPPPort == 443 {
		tlsConfig := &tls.Config{
			ServerName: w.settings.XMPPServer,
			NextProtos: []string{"xmpp-client"}, // ALPN protocol for C2S
		}
		options = append(options,
			xmpp.WithDirectTLS(),
			xmpp.WithClientTLS(tlsConfig),
		)
	}
	options = append(options, xmpp.WithConnectAddr(fmt.Sprintf("%s:%d", connectAddr, w.settings.XMPPPort)))

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
		return fmt.Errorf("connect to %s:%d: %w", connectAddr, w.settings.XMPPPort, err)
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
