package queries

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/shurco/mycart/internal/models"
)

// CreateMessage inserts a new message
func (r *ResponderQueries) CreateMessage(ctx context.Context, msg *models.Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()[:15]
	}

	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO message (id, contact_id, content, direction, channel, delivery_status, read_status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.ContactID, msg.Content, msg.Direction, msg.Channel, msg.DeliveryStatus, msg.ReadStatus)

	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// Fetch created timestamp
	query := fmt.Sprintf(`SELECT %s FROM message WHERE id = ?`, r.DB.Dialect().Epoch("created"))
	var created sql.NullInt64
	err = r.DB.QueryRowContext(ctx, query, msg.ID).Scan(&created)
	if err != nil {
		return fmt.Errorf("fetch created timestamp: %w", err)
	}

	msg.Created = created.Int64
	return nil
}

// ListMessagesByCustomer returns all messages for a customer, ordered by created DESC
func (r *ResponderQueries) ListMessagesByCustomer(ctx context.Context, customerID string) ([]models.Message, error) {
	query := fmt.Sprintf(`
		SELECT m.id, m.contact_id, m.content, m.direction, m.channel,
		       m.delivery_status, m.read_status, %s
		FROM message m
		INNER JOIN customer_contact cc ON m.contact_id = cc.id
		WHERE cc.customer_id = ?
		ORDER BY m.created DESC
	`, r.DB.Dialect().Epoch("m.created"))

	rows, err := r.DB.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var created sql.NullInt64

		err := rows.Scan(
			&m.ID, &m.ContactID, &m.Content, &m.Direction, &m.Channel,
			&m.DeliveryStatus, &m.ReadStatus, &created,
		)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		m.Created = created.Int64
		messages = append(messages, m)
	}

	return messages, nil
}

// UpdateMessageThread materializes the latest message info for a customer
func (r *ResponderQueries) UpdateMessageThread(ctx context.Context, customerID string) error {
	// Get latest message for this customer
	query := fmt.Sprintf(`
		SELECT m.id, m.content, %s,
		       (SELECT COUNT(*) FROM message m2
		        INNER JOIN customer_contact cc2 ON m2.contact_id = cc2.id
		        WHERE cc2.customer_id = ? AND m2.read_status = false) as unread_count
		FROM message m
		INNER JOIN customer_contact cc ON m.contact_id = cc.id
		WHERE cc.customer_id = ?
		ORDER BY m.created DESC
		LIMIT 1
	`, r.DB.Dialect().Epoch("m.created"))

	var messageID, preview string
	var lastMessageAt sql.NullInt64
	var unreadCount int

	err := r.DB.QueryRowContext(ctx, query, customerID, customerID).Scan(
		&messageID, &preview, &lastMessageAt, &unreadCount,
	)
	if err != nil {
		return fmt.Errorf("fetch latest message: %w", err)
	}

	// Delete existing thread
	_, err = r.DB.ExecContext(ctx, `DELETE FROM message_thread WHERE customer_id = ?`, customerID)
	if err != nil {
		return fmt.Errorf("delete existing thread: %w", err)
	}

	// Insert new thread
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO message_thread (id, customer_id, last_message_at, last_message_preview, unread_count)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.New().String()[:15], customerID, lastMessageAt.Int64, preview, unreadCount)

	if err != nil {
		return fmt.Errorf("insert message thread: %w", err)
	}

	return nil
}

// ListMessageThreads returns all message threads with optional filters
func (r *ResponderQueries) ListMessageThreads(ctx context.Context, filters map[string]string) ([]models.ThreadListItem, int, error) {
	whereClause := ""
	args := []any{}

	if channel, ok := filters["channel"]; ok {
		whereClause = `
			WHERE EXISTS (
				SELECT 1 FROM message m
				INNER JOIN customer_contact cc ON m.contact_id = cc.id
				WHERE cc.customer_id = mt.customer_id AND m.channel = ?
			)
		`
		args = append(args, channel)
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM message_thread mt %s", whereClause)
	var total int
	err := r.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count threads: %w", err)
	}

	// Fetch threads with customer info
	query := fmt.Sprintf(`
		SELECT mt.customer_id, c.name, c.email, %s, mt.last_message_preview, mt.unread_count
		FROM message_thread mt
		INNER JOIN customer c ON mt.customer_id = c.id
		%s
		ORDER BY mt.last_message_at DESC
	`, r.DB.Dialect().Epoch("mt.last_message_at"), whereClause)

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query threads: %w", err)
	}
	defer rows.Close()

	var threads []models.ThreadListItem
	for rows.Next() {
		var t models.ThreadListItem
		var lastMessageAt sql.NullInt64

		err := rows.Scan(
			&t.CustomerID, &t.CustomerName, &t.CustomerEmail,
			&lastMessageAt, &t.LastMessagePreview, &t.UnreadCount,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan thread: %w", err)
		}

		t.LastMessageAt = lastMessageAt.Int64

		// Fetch contacts for this customer
		contacts, _ := r.ListContactsByCustomer(ctx, t.CustomerID)
		t.Contacts = contacts

		threads = append(threads, t)
	}

	return threads, total, nil
}
