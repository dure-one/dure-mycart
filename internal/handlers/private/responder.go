package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
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
