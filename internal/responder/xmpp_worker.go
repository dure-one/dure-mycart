package responder

import (
	"context"
	"fmt"
	"strings"
	"time"

	xmpp "github.com/meszmate/xmpp-go"
	"github.com/meszmate/xmpp-go/jid"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
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

	// Create client with TLS (enabled by default)
	client, err := xmpp.NewClient(
		userJID,
		w.settings.XMPPPassword,
		xmpp.WithConnectAddr(fmt.Sprintf("%s:%d", w.settings.XMPPServer, w.settings.XMPPPort)),
	)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	// Connect and authenticate
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect to %s:%d: %w", w.settings.XMPPServer, w.settings.XMPPPort, err)
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
