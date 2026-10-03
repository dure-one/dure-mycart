package queries

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dure-one/dure-mycart/internal/models"
)

func TestGetOrCreateContact(t *testing.T) {
	db, ctx := bootstrap(t)

	// First call: creates customer + contact
	contact1, customer1, err := db.GetOrCreateContact(ctx, "sms", "+12125551234")
	require.NoError(t, err)
	assert.NotNil(t, contact1)
	assert.NotNil(t, customer1)
	assert.Equal(t, "sms", contact1.Type)
	assert.Equal(t, "+12125551234", contact1.Address)

	// Second call: returns existing
	contact2, customer2, err := db.GetOrCreateContact(ctx, "sms", "+12125551234")
	require.NoError(t, err)
	assert.Equal(t, contact1.ID, contact2.ID)
	assert.Equal(t, customer1.ID, customer2.ID)

	// Different address: creates new
	contact3, customer3, err := db.GetOrCreateContact(ctx, "xmpp", "user@example.com")
	require.NoError(t, err)
	assert.NotEqual(t, contact1.ID, contact3.ID)
	assert.NotEqual(t, customer1.ID, customer3.ID)
}

func TestLinkContactToCustomer(t *testing.T) {
	db, ctx := bootstrap(t)

	// Setup: create 2 customers with contacts
	contact1, customer1, _ := db.GetOrCreateContact(ctx, "sms", "+12125551234")
	_, customer2, _ := db.GetOrCreateContact(ctx, "sms", "+12125555678")

	// Link contact1 to customer2
	err := db.LinkContactToCustomer(ctx, contact1.ID, customer2.ID)
	require.NoError(t, err)

	// Verify contact updated
	contacts, err := db.ListContactsByCustomer(ctx, customer2.ID)
	require.NoError(t, err)
	assert.Len(t, contacts, 2) // customer2 now has 2 contacts

	found := false
	for _, c := range contacts {
		if c.ID == contact1.ID {
			found = true
			assert.Equal(t, customer2.ID, c.CustomerID)
		}
	}
	assert.True(t, found, "contact1 should be linked to customer2")

	// Verify customer1 no longer has contact1
	contacts1, err := db.ListContactsByCustomer(ctx, customer1.ID)
	require.NoError(t, err)
	assert.Len(t, contacts1, 0)
}

func TestListContactsByCustomer(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create customer with multiple contacts
	_, customer1, _ := db.GetOrCreateContact(ctx, "sms", "+12125551234")

	// Manually create second contact for same customer
	contact2 := &models.CustomerContact{
		CustomerID: customer1.ID,
		Type:       "xmpp",
		Address:    "user@example.com",
		Active:     true,
	}
	_, err := db.ResponderQueries.DB.ExecContext(ctx, `INSERT INTO customer_contact (id, customer_id, type, address, active) VALUES (?, ?, ?, ?, ?)`,
		"contact2id", contact2.CustomerID, contact2.Type, contact2.Address, contact2.Active)
	require.NoError(t, err)

	// List contacts
	contacts, err := db.ListContactsByCustomer(ctx, customer1.ID)
	require.NoError(t, err)
	assert.Len(t, contacts, 2)

	addresses := make([]string, len(contacts))
	for i, c := range contacts {
		addresses[i] = c.Address
	}
	assert.Contains(t, addresses, "+12125551234")
	assert.Contains(t, addresses, "user@example.com")
}
