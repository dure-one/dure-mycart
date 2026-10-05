package models

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// CustomerContact links a customer to their SMS or XMPP address
type CustomerContact struct {
	Core
	CustomerID string `json:"customer_id"`
	Type       string `json:"type"`    // sms, xmpp
	Address    string `json:"address"` // phone number or JID
	Active     bool   `json:"active"`
}

// Message is a single SMS or XMPP message
type Message struct {
	Core
	ContactID      string  `json:"contact_id"`
	Content        string  `json:"content"`
	Direction      string  `json:"direction"`       // inbound, outbound
	Channel        string  `json:"channel"`         // sms, xmpp
	DeliveryStatus *string `json:"delivery_status"` // pending, delivered, failed
	ReadStatus     bool    `json:"read_status"`
}

// MessageThread is a materialized view of message history per customer
type MessageThread struct {
	Core
	CustomerID         string `json:"customer_id"`
	LastMessageAt      int64  `json:"last_message_at"`
	LastMessagePreview string `json:"last_message_preview"`
	UnreadCount        int    `json:"unread_count"`
}

// Workflow is a mermaid markdown documentation flow
type Workflow struct {
	Core
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"` // mermaid markdown
	Enabled     bool   `json:"enabled"`
	Tags        string `json:"tags"`    // JSON array
	Version     string `json:"version"`
	Author      string `json:"author"`
}

// Validate workflow fields
func (w Workflow) Validate() error {
	return validation.ValidateStruct(&w,
		validation.Field(&w.Name, validation.Required, validation.Length(1, 200)),
		validation.Field(&w.Content, validation.Required),
		validation.Field(&w.Description, validation.Length(0, 1000)),
		validation.Field(&w.Version, validation.Length(0, 50)),
		validation.Field(&w.Author, validation.Length(0, 100)),
	)
}

// CrontabJob represents a predefined background job
type CrontabJob struct {
	Core
	JobType  string `json:"job_type"`  // xmpp_check, cleanup_inactive
	Interval string `json:"interval"`  // 5min, 15min, 1hr, 6hr, daily
	Enabled  bool   `json:"enabled"`
	LastRun  *int64 `json:"last_run"`
	NextRun  *int64 `json:"next_run"`
}

// MessageCreate is the request payload for creating a message
type MessageCreate struct {
	ContactAddress string `json:"contact_address"`
	ContactType    string `json:"contact_type"`
	Content        string `json:"content"`
	Direction      string `json:"direction"`
	Channel        string `json:"channel"`
	DeliveryStatus string `json:"delivery_status"`
}

// Validate message creation request
func (m MessageCreate) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.ContactAddress, validation.Required),
		validation.Field(&m.ContactType, validation.Required, validation.In("sms", "xmpp")),
		validation.Field(&m.Content, validation.Required, validation.Length(1, 10000)),
		validation.Field(&m.Direction, validation.Required, validation.In("inbound", "outbound")),
		validation.Field(&m.Channel, validation.Required, validation.In("sms", "xmpp")),
		validation.Field(&m.DeliveryStatus, validation.In("", "pending", "delivered", "failed")),
	)
}

// LinkContactRequest reassigns a contact to a different customer
type LinkContactRequest struct {
	ContactID     string `json:"contact_id"`
	NewCustomerID string `json:"new_customer_id"`
}

// Validate link contact request
func (l LinkContactRequest) Validate() error {
	return validation.ValidateStruct(&l,
		validation.Field(&l.ContactID, validation.Required),
		validation.Field(&l.NewCustomerID, validation.Required),
	)
}

// ResponderSettings holds XMPP configuration
type ResponderSettings struct {
	Enabled          bool   `json:"enabled"`            // Enable/disable XMPP connection
	XMPPJID          string `json:"xmpp_jid"`
	XMPPPassword     string `json:"xmpp_password"`
	XMPPServer       string `json:"xmpp_server"`        // Optional: defaults to JID domain (e.g., dure.co)
	XMPPPort         int    `json:"xmpp_port"`          // Optional: defaults to 443 for direct-tls, 5222 for starttls
	XMPPWebSocketURL string `json:"xmpp_websocket_url"` // Optional: WebSocket URL (e.g., wss://example.com/xmpp-websocket)
	XMPPBOSHURL      string `json:"xmpp_bosh_url"`      // Optional: BOSH endpoint URL (e.g., https://example.com/http-bind)
}

// Validate responder settings
func (r ResponderSettings) Validate() error {
	return validation.ValidateStruct(&r,
		// XMPPPort only validated if provided (non-zero)
		validation.Field(&r.XMPPPort, validation.When(r.XMPPPort != 0, validation.Min(1), validation.Max(65535))),
	)
}

// ThreadListItem is a thread summary for the list view
type ThreadListItem struct {
	CustomerID         string            `json:"customer_id"`
	CustomerName       string            `json:"customer_name"`
	CustomerEmail      string            `json:"customer_email"`
	LastMessageAt      int64             `json:"last_message_at"`
	LastMessagePreview string            `json:"last_message_preview"`
	UnreadCount        int               `json:"unread_count"`
	Contacts           []CustomerContact `json:"contacts"`
}
