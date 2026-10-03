package queries

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dure-one/dure-mycart/internal/models"
)

func TestCreateWorkflow(t *testing.T) {
	db, ctx := bootstrap(t)

	workflow := &models.Workflow{
		Name:        "Order Processing",
		Description: "Process customer orders",
		Content:     "graph TD\nA --> B",
		Enabled:     true,
		Tags:        `["orders", "automation"]`,
		Version:     "1.0",
		Author:      "admin",
	}

	err := db.CreateWorkflow(ctx, workflow)
	require.NoError(t, err)
	assert.NotEmpty(t, workflow.ID)
	assert.NotZero(t, workflow.Created)
}

func TestGetWorkflow(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create workflow first
	workflow := &models.Workflow{
		Name:    "Test Workflow",
		Content: "graph TD\nA --> B",
		Enabled: true,
	}
	db.CreateWorkflow(ctx, workflow)

	// Get workflow
	fetched, err := db.GetWorkflow(ctx, workflow.ID)
	require.NoError(t, err)
	assert.Equal(t, workflow.ID, fetched.ID)
	assert.Equal(t, "Test Workflow", fetched.Name)
}

func TestListWorkflows(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create multiple workflows
	for i := 0; i < 3; i++ {
		workflow := &models.Workflow{
			Name:    "Workflow",
			Content: "graph TD\nA --> B",
			Enabled: i%2 == 0,
		}
		db.CreateWorkflow(ctx, workflow)
	}

	// List all
	workflows, total, err := db.ListWorkflows(ctx, map[string]string{}, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 3)
	assert.GreaterOrEqual(t, len(workflows), 3)

	// Filter by enabled
	workflows, total, err = db.ListWorkflows(ctx, map[string]string{"enabled": "true"}, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 2)
}

func TestUpdateWorkflow(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create workflow
	workflow := &models.Workflow{
		Name:    "Original",
		Content: "graph TD\nA --> B",
		Enabled: true,
	}
	db.CreateWorkflow(ctx, workflow)

	// Update workflow
	workflow.Name = "Updated"
	workflow.Enabled = false
	err := db.UpdateWorkflow(ctx, workflow)
	require.NoError(t, err)

	// Verify update
	fetched, _ := db.GetWorkflow(ctx, workflow.ID)
	assert.Equal(t, "Updated", fetched.Name)
	assert.False(t, fetched.Enabled)
}

func TestDeleteWorkflow(t *testing.T) {
	db, ctx := bootstrap(t)

	// Create workflow
	workflow := &models.Workflow{
		Name:    "To Delete",
		Content: "graph TD\nA --> B",
		Enabled: true,
	}
	db.CreateWorkflow(ctx, workflow)

	// Delete workflow
	err := db.DeleteWorkflow(ctx, workflow.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = db.GetWorkflow(ctx, workflow.ID)
	assert.Error(t, err)
}
