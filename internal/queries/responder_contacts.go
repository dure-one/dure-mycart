package queries

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
)

// GetOrCreateContact returns existing contact or creates customer + contact if unknown address
func (r *ResponderQueries) GetOrCreateContact(ctx context.Context, contactType, address string) (*models.CustomerContact, *models.Customer, error) {
	// Try to find existing contact
	var contact models.CustomerContact
	var created, updated sql.NullInt64

	query := fmt.Sprintf(`
		SELECT id, customer_id, type, address, active, %s, %s
		FROM customer_contact
		WHERE type = ? AND address = ?
	`, r.DB.Dialect().Epoch("created"), r.DB.Dialect().Epoch("updated"))

	err := r.DB.QueryRowContext(ctx, query, contactType, address).Scan(
		&contact.ID,
		&contact.CustomerID,
		&contact.Type,
		&contact.Address,
		&contact.Active,
		&created,
		&updated,
	)

	if err == nil {
		// Contact exists, assign timestamps and fetch customer
		contact.Created = created.Int64
		contact.Updated = updated.Int64

		customer, err := DB().CustomerByID(ctx, contact.CustomerID)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch customer: %w", err)
		}
		return &contact, customer, nil
	}

	if err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("query contact: %w", err)
	}

	// Contact doesn't exist, create customer + contact
	customerID := uuid.New().String()[:15]
	contactID := uuid.New().String()[:15]

	// Create customer with placeholder email
	customerEmail := fmt.Sprintf("%s@responder.local", contactID)
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO customer (id, email, password, name, active)
		VALUES (?, ?, '', '', true)
	`, customerID, customerEmail)
	if err != nil {
		return nil, nil, fmt.Errorf("create customer: %w", err)
	}

	// Create contact
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO customer_contact (id, customer_id, type, address, active)
		VALUES (?, ?, ?, ?, true)
	`, contactID, customerID, contactType, address)
	if err != nil {
		return nil, nil, fmt.Errorf("create contact: %w", err)
	}

	// Fetch created records
	customer, err := DB().CustomerByID(ctx, customerID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch created customer: %w", err)
	}

	contact = models.CustomerContact{
		Core:       models.Core{ID: contactID},
		CustomerID: customerID,
		Type:       contactType,
		Address:    address,
		Active:     true,
	}

	return &contact, customer, nil
}

// LinkContactToCustomer reassigns a contact to a different customer
func (r *ResponderQueries) LinkContactToCustomer(ctx context.Context, contactID, newCustomerID string) error {
	result, err := r.DB.ExecContext(ctx, `
		UPDATE customer_contact
		SET customer_id = ?, updated = CURRENT_TIMESTAMP
		WHERE id = ?
	`, newCustomerID, contactID)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return errors.ErrContactNotFound
	}

	return nil
}

// ListContactsByCustomer returns all contacts for a customer
func (r *ResponderQueries) ListContactsByCustomer(ctx context.Context, customerID string) ([]models.CustomerContact, error) {
	query := fmt.Sprintf(`
		SELECT id, customer_id, type, address, active, %s, %s
		FROM customer_contact
		WHERE customer_id = ?
		ORDER BY created DESC
	`, r.DB.Dialect().Epoch("created"), r.DB.Dialect().Epoch("updated"))

	rows, err := r.DB.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	var contacts []models.CustomerContact
	for rows.Next() {
		var c models.CustomerContact
		var created, updated sql.NullInt64

		err := rows.Scan(&c.ID, &c.CustomerID, &c.Type, &c.Address, &c.Active, &created, &updated)
		if err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}

		c.Created = created.Int64
		c.Updated = updated.Int64
		contacts = append(contacts, c)
	}

	return contacts, nil
}
