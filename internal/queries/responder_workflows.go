package queries

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
)

// ListWorkflows returns all workflows with optional filtering
func (r *ResponderQueries) ListWorkflows(ctx context.Context, filters map[string]string, limit, offset int) ([]models.Workflow, int, error) {
	whereClause := ""
	args := []any{}

	if enabled, ok := filters["enabled"]; ok {
		whereClause = "WHERE enabled = ?"
		args = append(args, enabled == "true")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM workflow %s", whereClause)
	var total int
	err := r.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count workflows: %w", err)
	}

	// Fetch workflows
	query := fmt.Sprintf(`
		SELECT id, name, description, content, enabled, tags, version, author, %s, %s
		FROM workflow
		%s
		ORDER BY created DESC
		LIMIT ? OFFSET ?
	`, r.DB.Dialect().Epoch("created"), r.DB.Dialect().Epoch("updated"), whereClause)

	args = append(args, limit, offset)
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query workflows: %w", err)
	}
	defer rows.Close()

	var workflows []models.Workflow
	for rows.Next() {
		var w models.Workflow
		var created, updated sql.NullInt64

		err := rows.Scan(
			&w.ID, &w.Name, &w.Description, &w.Content, &w.Enabled,
			&w.Tags, &w.Version, &w.Author, &created, &updated,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan workflow: %w", err)
		}

		w.Created = created.Int64
		w.Updated = updated.Int64
		workflows = append(workflows, w)
	}

	return workflows, total, nil
}

// GetWorkflow returns a single workflow by ID
func (r *ResponderQueries) GetWorkflow(ctx context.Context, workflowID string) (*models.Workflow, error) {
	var w models.Workflow
	var created, updated sql.NullInt64

	query := fmt.Sprintf(`
		SELECT id, name, description, content, enabled, tags, version, author, %s, %s
		FROM workflow
		WHERE id = ?
	`, r.DB.Dialect().Epoch("created"), r.DB.Dialect().Epoch("updated"))

	err := r.DB.QueryRowContext(ctx, query, workflowID).Scan(
		&w.ID, &w.Name, &w.Description, &w.Content, &w.Enabled,
		&w.Tags, &w.Version, &w.Author, &created, &updated,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("query workflow: %w", err)
	}

	w.Created = created.Int64
	w.Updated = updated.Int64

	return &w, nil
}

// CreateWorkflow inserts a new workflow
func (r *ResponderQueries) CreateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	if workflow.ID == "" {
		workflow.ID = uuid.New().String()[:15]
	}

	if err := workflow.Validate(); err != nil {
		return err
	}

	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO workflow (id, name, description, content, enabled, tags, version, author)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, workflow.ID, workflow.Name, workflow.Description, workflow.Content, workflow.Enabled,
		workflow.Tags, workflow.Version, workflow.Author)

	if err != nil {
		return fmt.Errorf("insert workflow: %w", err)
	}

	// Fetch created timestamp
	query := fmt.Sprintf(`SELECT %s FROM workflow WHERE id = ?`, r.DB.Dialect().Epoch("created"))
	var created sql.NullInt64
	err = r.DB.QueryRowContext(ctx, query, workflow.ID).Scan(&created)
	if err != nil {
		return fmt.Errorf("fetch created timestamp: %w", err)
	}

	workflow.Created = created.Int64
	return nil
}

// UpdateWorkflow updates an existing workflow
func (r *ResponderQueries) UpdateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	if err := workflow.Validate(); err != nil {
		return err
	}

	result, err := r.DB.ExecContext(ctx, `
		UPDATE workflow
		SET name = ?, description = ?, content = ?, enabled = ?, tags = ?, version = ?, author = ?, updated = CURRENT_TIMESTAMP
		WHERE id = ?
	`, workflow.Name, workflow.Description, workflow.Content, workflow.Enabled,
		workflow.Tags, workflow.Version, workflow.Author, workflow.ID)

	if err != nil {
		return fmt.Errorf("update workflow: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return errors.ErrNotFound
	}

	return nil
}

// DeleteWorkflow deletes a workflow by ID
func (r *ResponderQueries) DeleteWorkflow(ctx context.Context, workflowID string) error {
	result, err := r.DB.ExecContext(ctx, `DELETE FROM workflow WHERE id = ?`, workflowID)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return errors.ErrNotFound
	}

	return nil
}
