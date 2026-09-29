package responder

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/xmppo/go-xmpp"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
)

// XMPPWorker handles XMPP message synchronization via MAM
type XMPPWorker struct {
	settings *models.ResponderSettings
	db       *queries.Base
	conn     *xmpp.Client
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

// connect establishes XMPP connection
func (w *XMPPWorker) connect() error {
	options := xmpp.Options{
		Host:     fmt.Sprintf("%s:%d", w.settings.XMPPServer, w.settings.XMPPPort),
		User:     w.settings.XMPPJID,
		Password: w.settings.XMPPPassword,
		NoTLS:    false,
		TLSConfig: &tls.Config{
			ServerName: w.settings.XMPPServer,
		},
	}

	client, err := options.NewClient()
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	w.conn = client
	return nil
}

// disconnect closes the XMPP connection
func (w *XMPPWorker) disconnect() {
	if w.conn != nil {
		_ = w.conn.Close()
	}
}

// fetchMessages queries MAM and syncs messages to database
// ponytail: basic MAM stub - extend with XEP-0313 when MAM server available
func (w *XMPPWorker) fetchMessages(ctx context.Context) error {
	if w.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Send MAM query (XEP-0313)
	mamIQ := fmt.Sprintf(`<iq type='set' id='mam1'>
		<query xmlns='urn:xmpp:mam:2'>
			<x xmlns='jabber:x:data' type='submit'>
				<field var='FORM_TYPE' type='hidden'>
					<value>urn:xmpp:mam:2</value>
				</field>
				<field var='start'>
					<value>%s</value>
				</field>
			</x>
		</query>
	</iq>`, w.lastSync.UTC().Format(time.RFC3339))

	_, err := w.conn.SendOrg(mamIQ)
	if err != nil {
		return fmt.Errorf("send mam query: %w", err)
	}

	// Read messages for 2 seconds
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			w.lastSync = time.Now()
			return nil
		default:
			msg, err := w.conn.Recv()
			if err != nil {
				return nil // No more messages
			}

			chat, ok := msg.(xmpp.Chat)
			if !ok {
				continue // Not a chat message
			}

			if err := w.processChat(ctx, chat); err != nil {
				return fmt.Errorf("process chat: %w", err)
			}
		}
	}
}

// processChat processes incoming XMPP chat message
func (w *XMPPWorker) processChat(ctx context.Context, chat xmpp.Chat) error {
	if chat.Text == "" {
		return nil
	}

	// Extract bare JID (remove resource)
	from := chat.Remote
	if idx := strings.Index(from, "/"); idx > 0 {
		from = from[:idx]
	}

	contact, _, err := w.db.GetOrCreateContact(ctx, "xmpp", from)
	if err != nil {
		return fmt.Errorf("get or create contact: %w", err)
	}

	message := &models.Message{
		ContactID: contact.ID,
		Content:   chat.Text,
		Direction: "inbound",
		Channel:   "xmpp",
	}

	if err := w.db.CreateMessage(ctx, message); err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	return nil
}
