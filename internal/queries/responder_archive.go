package queries

import (
	"context"
	"fmt"
	"time"
)

// ArchiveInactiveCustomers moves customers with no activity since inactiveSince to archived tables.
// Returns the number of customers archived.
func (r *ResponderQueries) ArchiveInactiveCustomers(ctx context.Context, inactiveSince time.Time) (int, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	inactiveSinceEpoch := inactiveSince.Unix()

	// Find inactive customers (no messages since inactiveSince)
	epochExpr := r.DB.Dialect().Epoch("m.created")
	query := fmt.Sprintf(`
		SELECT DISTINCT c.id
		FROM customer c
		INNER JOIN customer_contact cc ON c.id = cc.customer_id
		LEFT JOIN message m ON cc.id = m.contact_id
		GROUP BY c.id
		HAVING COALESCE(CAST(MAX(%s) AS INTEGER), 0) < ?
	`, epochExpr)

	rows, err := tx.QueryContext(ctx, query, inactiveSinceEpoch)
	if err != nil {
		return 0, fmt.Errorf("query inactive customers: %w", err)
	}
	defer rows.Close()

	var customerIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan customer id: %w", err)
		}
		customerIDs = append(customerIDs, id)
	}

	if len(customerIDs) == 0 {
		return 0, nil
	}

	// For each customer, archive their data
	for _, customerID := range customerIDs {
		// Copy customer to archived_customer
		_, err = tx.ExecContext(ctx, `
			INSERT INTO archived_customer (id, email, password, name, active, created, updated, archived_at)
			SELECT id, email, password, name, active, created, updated, CURRENT_TIMESTAMP
			FROM customer
			WHERE id = ?
		`, customerID)
		if err != nil {
			return 0, fmt.Errorf("archive customer %s: %w", customerID, err)
		}

		// Copy messages to archived_message
		_, err = tx.ExecContext(ctx, `
			INSERT INTO archived_message (id, contact_id, content, direction, channel, delivery_status, read_status, created, archived_at)
			SELECT m.id, m.contact_id, m.content, m.direction, m.channel, m.delivery_status, m.read_status, m.created, CURRENT_TIMESTAMP
			FROM message m
			INNER JOIN customer_contact cc ON m.contact_id = cc.id
			WHERE cc.customer_id = ?
		`, customerID)
		if err != nil {
			return 0, fmt.Errorf("archive messages for customer %s: %w", customerID, err)
		}

		// Delete message_thread
		_, err = tx.ExecContext(ctx, `DELETE FROM message_thread WHERE customer_id = ?`, customerID)
		if err != nil {
			return 0, fmt.Errorf("delete message_thread for customer %s: %w", customerID, err)
		}

		// Delete messages (via CASCADE from customer_contact)
		// Delete customer_contact (via CASCADE from customer)
		// Delete customer
		_, err = tx.ExecContext(ctx, `DELETE FROM customer WHERE id = ?`, customerID)
		if err != nil {
			return 0, fmt.Errorf("delete customer %s: %w", customerID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return len(customerIDs), nil
}
