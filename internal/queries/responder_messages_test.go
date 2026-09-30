package queries

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shurco/mycart/internal/models"
)

func TestCreateMessage(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create contact first
	contact, _, _ := db.GetOrCreateContact(ctx, "sms", "+12125551234")

	// Create message
	msg := &models.Message{
		ContactID:      contact.ID,
		Content:        "Test message",
		Direction:      "inbound",
		Channel:        "sms",
		DeliveryStatus: nil,
		ReadStatus:     false,
	}

	err := db.CreateMessage(ctx, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, msg.ID)
	assert.NotZero(t, msg.Created)
}

func TestListMessagesByCustomer(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create contact and customer
	contact, customer, _ := db.GetOrCreateContact(ctx, "sms", "+12125551234")

	// Create multiple messages
	for i := 0; i < 3; i++ {
		msg := &models.Message{
			ContactID: contact.ID,
			Content:   "Test message",
			Direction: "inbound",
			Channel:   "sms",
		}
		db.CreateMessage(ctx, msg)
	}

	// List messages
	messages, err := db.ListMessagesByCustomer(ctx, customer.ID)
	require.NoError(t, err)
	assert.Len(t, messages, 3)
}

func TestUpdateMessageThread(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create contact and customer
	contact, customer, _ := db.GetOrCreateContact(ctx, "sms", "+12125551234")

	// Create message
	msg := &models.Message{
		ContactID: contact.ID,
		Content:   "Test message",
		Direction: "inbound",
		Channel:   "sms",
	}
	db.CreateMessage(ctx, msg)

	// Update thread
	err := db.UpdateMessageThread(ctx, customer.ID)
	require.NoError(t, err)

	// Verify thread exists
	threads, _, err := db.ListMessageThreads(ctx, map[string]string{})
	require.NoError(t, err)
	assert.Len(t, threads, 1)
	assert.Equal(t, customer.ID, threads[0].CustomerID)
	assert.Equal(t, "Test message", threads[0].LastMessagePreview)
}

func TestListMessageThreads(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create two customers with messages
	contact1, customer1, _ := db.GetOrCreateContact(ctx, "sms", "+12125551234")
	contact2, customer2, _ := db.GetOrCreateContact(ctx, "xmpp", "user@example.com")

	msg1 := &models.Message{
		ContactID: contact1.ID,
		Content:   "SMS message",
		Direction: "inbound",
		Channel:   "sms",
	}
	db.CreateMessage(ctx, msg1)
	db.UpdateMessageThread(ctx, customer1.ID)

	msg2 := &models.Message{
		ContactID: contact2.ID,
		Content:   "XMPP message",
		Direction: "inbound",
		Channel:   "xmpp",
	}
	db.CreateMessage(ctx, msg2)
	db.UpdateMessageThread(ctx, customer2.ID)

	// List all threads
	threads, total, err := db.ListMessageThreads(ctx, map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, threads, 2)

	// Filter by channel
	threads, total, err = db.ListMessageThreads(ctx, map[string]string{"channel": "sms"})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, threads, 1)
	assert.Equal(t, "SMS message", threads[0].LastMessagePreview)
}
