package handlers

import (
	"os"

	"github.com/gofiber/fiber/v3"

	"github.com/dure-one/dure-mycart/internal/models"
	"github.com/dure-one/dure-mycart/internal/queries"
	"github.com/dure-one/dure-mycart/internal/responder"
	"github.com/dure-one/dure-mycart/pkg/errors"
	"github.com/dure-one/dure-mycart/pkg/logging"
	"github.com/dure-one/dure-mycart/pkg/webutil"
)

// MessageThreads returns a list of message threads with optional channel filter.
//
// @Summary      List message threads
// @Description  Get list of message threads grouped by customer
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Param        channel query string false "Filter by channel (sms, xmpp)"
// @Success      200 {object} webutil.HTTPResponse "Message threads list"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/messages [get]
func MessageThreads(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	filters := make(map[string]string)
	if channel := c.Query("channel"); channel != "" {
		filters["channel"] = channel
	}

	threads, total, err := db.ListMessageThreads(c.Context(), filters)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Message threads", map[string]any{
		"threads": threads,
		"total":   total,
	})
}

// MessagesByCustomer returns all messages for a specific customer.
//
// @Summary      Get customer messages
// @Description  Get all messages for a specific customer ordered by created DESC
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Param        customer_id path string true "Customer ID"
// @Success      200 {object} webutil.HTTPResponse "Customer messages"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/messages/{customer_id} [get]
func MessagesByCustomer(c fiber.Ctx) error {
	customerID := c.Params("customer_id")
	db := queries.DB()
	log := logging.New()

	messages, err := db.ListMessagesByCustomer(c.Context(), customerID)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Customer messages", map[string]any{
		"messages": messages,
	})
}

// CreateMessage creates a new message and auto-creates customer/contact if needed.
//
// @Summary      Create message
// @Description  Create a new message (auto-creates customer and contact on unknown address)
// @Tags         Responder
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body models.MessageCreate true "Message data"
// @Success      200 {object} webutil.HTTPResponse{result=models.Message} "Created message"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/messages [post]
func CreateMessage(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()
	request := new(models.MessageCreate)

	if err := c.Bind().Body(request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := request.Validate(); err != nil {
		return webutil.StatusBadRequest(c, err.Error())
	}

	// Get or create contact
	contact, customer, err := db.GetOrCreateContact(c.Context(), request.ContactType, request.ContactAddress)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Create message
	var deliveryStatus *string
	if request.DeliveryStatus != "" {
		deliveryStatus = &request.DeliveryStatus
	}

	msg := &models.Message{
		ContactID:      contact.ID,
		Content:        request.Content,
		Direction:      request.Direction,
		Channel:        request.Channel,
		DeliveryStatus: deliveryStatus,
		ReadStatus:     false,
	}

	if err := db.CreateMessage(c.Context(), msg); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Update message thread
	if err := db.UpdateMessageThread(c.Context(), customer.ID); err != nil {
		log.ErrorStack(err)
		// Don't fail the request, just log the error
	}

	return webutil.Response(c, fiber.StatusOK, "Message created", msg)
}

// MarkMessageRead marks a message as read.
//
// @Summary      Mark message read
// @Description  Mark a message as read by its ID
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Param        message_id path string true "Message ID"
// @Success      200 {object} webutil.HTTPResponse "Message marked as read"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/messages/{message_id}/read [patch]
func MarkMessageRead(c fiber.Ctx) error {
	messageID := c.Params("message_id")
	db := queries.DB()
	log := logging.New()

	if err := db.MarkMessageRead(c.Context(), messageID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Message marked as read", nil)
}

// LinkContact reassigns a contact to a different customer.
//
// @Summary      Link contact to customer
// @Description  Reassign a contact to a different customer
// @Tags         Responder
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "Contact ID and new customer ID"
// @Success      200 {object} webutil.HTTPResponse "Contact linked"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/messages/link-contact [post]
func LinkContact(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	var request struct {
		ContactID     string `json:"contact_id"`
		NewCustomerID string `json:"new_customer_id"`
	}

	if err := c.Bind().Body(&request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if request.ContactID == "" || request.NewCustomerID == "" {
		return webutil.StatusBadRequest(c, "contact_id and new_customer_id are required")
	}

	if err := db.LinkContactToCustomer(c.Context(), request.ContactID, request.NewCustomerID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Contact linked to customer", nil)
}

// Workflows returns a list of workflows with optional filtering.
//
// @Summary      List workflows
// @Description  Get paginated list of workflows with optional enabled filter
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Param        enabled query string false "Filter by enabled status (true/false)"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200 {object} webutil.HTTPResponse "Workflows list"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/workflows [get]
func Workflows(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	p := webutil.ParsePagination(c)
	filters := make(map[string]string)
	if enabled := c.Query("enabled"); enabled != "" {
		filters["enabled"] = enabled
	}

	workflows, total, err := db.ListWorkflows(c.Context(), filters, p.Limit, p.Offset)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Workflows", map[string]any{
		"workflows": workflows,
		"total":     total,
		"page":      p.Page,
		"limit":     p.Limit,
	})
}

// GetWorkflow returns a single workflow by ID.
//
// @Summary      Get workflow
// @Description  Get a single workflow by its ID
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Param        workflow_id path string true "Workflow ID"
// @Success      200 {object} webutil.HTTPResponse{result=models.Workflow} "Workflow details"
// @Failure      404 {object} webutil.HTTPResponse "Workflow not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/workflows/{workflow_id} [get]
func GetWorkflow(c fiber.Ctx) error {
	workflowID := c.Params("workflow_id")
	db := queries.DB()
	log := logging.New()

	workflow, err := db.GetWorkflow(c.Context(), workflowID)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Workflow", workflow)
}

// CreateWorkflow creates a new workflow.
//
// @Summary      Create workflow
// @Description  Create a new workflow with mermaid content
// @Tags         Responder
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body models.Workflow true "Workflow data"
// @Success      200 {object} webutil.HTTPResponse{result=models.Workflow} "Created workflow"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/workflows [post]
func CreateWorkflow(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()
	workflow := new(models.Workflow)

	if err := c.Bind().Body(workflow); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := db.CreateWorkflow(c.Context(), workflow); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Workflow created", workflow)
}

// UpdateWorkflow updates an existing workflow.
//
// @Summary      Update workflow
// @Description  Update workflow details
// @Tags         Responder
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        workflow_id path string true "Workflow ID"
// @Param        request body models.Workflow true "Workflow data"
// @Success      200 {object} webutil.HTTPResponse "Workflow updated"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      404 {object} webutil.HTTPResponse "Workflow not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/workflows/{workflow_id} [patch]
func UpdateWorkflow(c fiber.Ctx) error {
	workflowID := c.Params("workflow_id")
	db := queries.DB()
	log := logging.New()
	workflow := new(models.Workflow)
	workflow.ID = workflowID

	if err := c.Bind().Body(workflow); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := db.UpdateWorkflow(c.Context(), workflow); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Workflow updated", nil)
}

// DeleteWorkflow deletes a workflow by ID.
//
// @Summary      Delete workflow
// @Description  Delete a workflow by its ID
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Param        workflow_id path string true "Workflow ID"
// @Success      200 {object} webutil.HTTPResponse "Workflow deleted"
// @Failure      404 {object} webutil.HTTPResponse "Workflow not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/responder/workflows/{workflow_id} [delete]
func DeleteWorkflow(c fiber.Ctx) error {
	workflowID := c.Params("workflow_id")
	db := queries.DB()
	log := logging.New()

	if err := db.DeleteWorkflow(c.Context(), workflowID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Workflow deleted", nil)
}

// CrontabJobs returns all crontab jobs.
//
// @Summary      List crontab jobs
// @Description  Get list of all crontab jobs
// @Tags         Settings
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Crontab jobs list"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/crontab [get]
func CrontabJobs(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	jobs, err := db.ListCrontabJobs(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Crontab jobs", map[string]any{
		"jobs": jobs,
	})
}

// UpdateCrontabJobSettings updates a crontab job's enabled status and interval.
//
// @Summary      Update crontab job
// @Description  Update crontab job settings (enabled, interval)
// @Tags         Settings
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        job_id path string true "Job ID"
// @Param        request body map[string]any true "Job settings"
// @Success      200 {object} webutil.HTTPResponse "Job updated"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      404 {object} webutil.HTTPResponse "Job not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/crontab/{job_id} [patch]
func UpdateCrontabJobSettings(c fiber.Ctx) error {
	jobID := c.Params("job_id")
	db := queries.DB()
	log := logging.New()

	var request struct {
		Enabled  bool   `json:"enabled"`
		Interval string `json:"interval"`
	}

	if err := c.Bind().Body(&request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := db.UpdateCrontabJob(c.Context(), jobID, request.Enabled, request.Interval); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	return webutil.Response(c, fiber.StatusOK, "Crontab job updated", nil)
}

// GetResponderSettings returns responder XMPP settings.
//
// @Summary      Get responder settings
// @Description  Get XMPP configuration settings
// @Tags         Settings
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse{result=models.ResponderSettings} "Responder settings"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/responder [get]
func GetResponderSettings(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	// Fetch settings from database
	settings := &models.ResponderSettings{}
	result, err := db.GetSettingByGroup(c.Context(), settings)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Responder settings", result)
}

// UpdateResponderSettings updates responder XMPP settings.
//
// @Summary      Update responder settings
// @Description  Update XMPP configuration settings
// @Tags         Settings
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body models.ResponderSettings true "Responder settings"
// @Success      200 {object} webutil.HTTPResponse "Settings updated"
// @Failure      400 {object} webutil.HTTPResponse "Validation error"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/responder [patch]
func UpdateResponderSettings(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()
	settings := new(models.ResponderSettings)

	if err := c.Bind().Body(settings); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := settings.Validate(); err != nil {
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := db.UpdateSettingByGroup(c.Context(), settings); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Responder settings updated", nil)
}

// XMPPConnectionTest tests the XMPP connection with provided settings.
//
// @Summary      Test XMPP connection
// @Description  Test XMPP connection with current settings
// @Tags         Settings
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Connection successful"
// @Failure      500 {object} webutil.HTTPResponse "Connection failed"
// @Router       /api/_/settings/responder/test-connection [post]
func XMPPConnectionTest(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	var settings models.ResponderSettings
	if _, err := db.GetSettingByGroup(c.Context(), &settings); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if settings.XMPPJID == "" || settings.XMPPPassword == "" {
		return webutil.Response(c, fiber.StatusOK, "XMPP settings incomplete", map[string]any{
			"success": false,
			"error":   "XMPP JID and password required",
		})
	}

	worker := responder.NewXMPPWorker(&settings, nil)
	if err := worker.Connect(); err != nil {
		return webutil.Response(c, fiber.StatusOK, "XMPP connection failed", map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}
	defer worker.Disconnect()

	return webutil.Response(c, fiber.StatusOK, "XMPP connection successful", map[string]any{
		"success": true,
	})
}

// CheckCrontabStatus checks if system crontab is installed.
//
// @Summary      Check crontab status
// @Description  Check if mycart system crontab is installed
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Crontab status"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/crontab/status [get]
func CheckCrontabStatus(c fiber.Ctx) error {
	log := logging.New()

	binaryPath := getBinaryPath()
	manager := responder.NewCrontabManager(binaryPath)

	installed, err := manager.IsInstalled(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Crontab status", map[string]any{
		"installed": installed,
	})
}

// InstallCrontab installs system crontab with enabled jobs.
//
// @Summary      Install system crontab
// @Description  Install mycart crontab entries for enabled jobs
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Crontab installed"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/crontab/install [post]
func InstallCrontab(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	jobs, err := db.ListCrontabJobs(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	binaryPath := getBinaryPath()
	manager := responder.NewCrontabManager(binaryPath)

	if err := manager.Install(c.Context(), jobs); err != nil {
		log.ErrorStack(err)
		return webutil.Response(c, fiber.StatusOK, "Failed to install crontab", map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return webutil.Response(c, fiber.StatusOK, "Crontab installed successfully", map[string]any{
		"success": true,
	})
}

// UninstallCrontab removes mycart system crontab entries.
//
// @Summary      Uninstall system crontab
// @Description  Remove mycart crontab entries from system
// @Tags         Responder
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse "Crontab uninstalled"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/settings/crontab/uninstall [post]
func UninstallCrontab(c fiber.Ctx) error {
	log := logging.New()

	binaryPath := getBinaryPath()
	manager := responder.NewCrontabManager(binaryPath)

	if err := manager.Uninstall(c.Context()); err != nil {
		log.ErrorStack(err)
		return webutil.Response(c, fiber.StatusOK, "Failed to uninstall crontab", map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return webutil.Response(c, fiber.StatusOK, "Crontab uninstalled successfully", map[string]any{
		"success": true,
	})
}

// getBinaryPath returns the path to the current executable
func getBinaryPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "mycart" // fallback
	}
	return executable
}
