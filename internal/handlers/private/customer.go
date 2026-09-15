package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/security"
	"github.com/shurco/mycart/pkg/webutil"
)

// Customers lists the shop's customers.
//
// @Summary      List customers
// @Description  Get a paginated list of the shop's customers: registered accounts and everyone who has paid, one row per address
// @Tags         Customers
// @Security     BearerAuth
// @Produce      json
// @Param        page       query int    false "Page number" default(1)
// @Param        limit      query int    false "Items per page" default(20)
// @Param        search     query string false "Match a substring of the email address or name"
// @Param        registered query bool   false "Only customers with an account"
// @Success      200 {object} webutil.HTTPResponse{result=[]models.CustomerSummary} "Customers list with pagination"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/customers [get]
func Customers(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	currency, err := shopCurrency(c)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	p := webutil.ParsePagination(c)
	filter := queries.CustomerFilter{
		Search:         strings.TrimSpace(c.Query("search")),
		RegisteredOnly: parseBoolQuery(c, "registered"),
		Currency:       currency,
	}

	customers, total, err := db.Customers(c.Context(), filter, p.Limit, p.Offset)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Customers", map[string]any{
		"customers": customers,
		"total":     total,
		"page":      p.Page,
		"limit":     p.Limit,
	})
}

// CustomerCarts returns everything one customer has ordered.
//
// @Summary      Get customer carts
// @Description  Get every cart created for an email address, newest first, in any payment state
// @Tags         Customers
// @Security     BearerAuth
// @Produce      json
// @Param        email query string true "Customer email address"
// @Success      200 {object} webutil.HTTPResponse{result=[]models.Cart} "Customer carts"
// @Failure      400 {object} webutil.HTTPResponse "Missing email"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/customers/carts [get]
func CustomerCarts(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	// The address is the key rather than the account id, because most rows in
	// the customer list have no account behind them: a guest buyer is reachable
	// only through what they typed at checkout.
	email := queries.NormalizeEmail(c.Query("email"))
	if email == "" {
		return webutil.StatusBadRequest(c, "email is required")
	}

	carts, err := db.CartsByEmail(c.Context(), email)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Customer carts", map[string]any{
		"email": email,
		"carts": carts,
		"total": len(carts),
	})
}

// UpdateCustomerActive blocks or unblocks a customer account.
//
// @Summary      Toggle customer active
// @Description  Toggle the active status of a customer account; blocking also ends every session the customer has open
// @Tags         Customers
// @Security     BearerAuth
// @Produce      json
// @Param        customer_id path string true "Customer ID"
// @Success      200 {object} webutil.HTTPResponse{result=models.Customer} "Updated customer"
// @Failure      404 {object} webutil.HTTPResponse "Customer not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/customers/{customer_id}/active [patch]
func UpdateCustomerActive(c fiber.Ctx) error {
	customerID := c.Params("customer_id")
	db := queries.DB()
	log := logging.New()

	customer, err := db.CustomerByID(c.Context(), customerID)
	if err != nil {
		if errors.Is(err, errors.ErrCustomerNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	active := !customer.Active
	if err := db.SetCustomerActive(c.Context(), customerID, active); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Blocking has to take effect now, not when the session would have expired:
	// a token is honoured for as long as its session row lives, so the row is
	// what gets deleted. Unblocking revokes nothing — a blocked customer has no
	// session left to end.
	if !active {
		if err := db.RevokeCustomerSessions(c.Context(), customerID); err != nil {
			log.ErrorStack(err)
			return webutil.StatusInternalServerError(c)
		}
	}

	// The updated account is read back and returned, the way the page and
	// product toggles do it: the row carries the new status and the new
	// "updated" stamp, so the caller does not have to guess either.
	var updated *models.Customer
	if updated, err = db.CustomerByID(c.Context(), customerID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Customer status updated", updated)
}

// UpdateCustomerPassword issues a new password for a customer account.
//
// @Summary      Reset customer password
// @Description  Generate a new password for a customer account and return it once; every session the customer has open is ended
// @Tags         Customers
// @Security     BearerAuth
// @Produce      json
// @Param        customer_id path string true "Customer ID"
// @Success      200 {object} webutil.HTTPResponse "New password"
// @Failure      404 {object} webutil.HTTPResponse "Customer not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/customers/{customer_id}/password [patch]
func UpdateCustomerPassword(c fiber.Ctx) error {
	customerID := c.Params("customer_id")
	db := queries.DB()
	log := logging.New()

	if _, err := db.CustomerByID(c.Context(), customerID); err != nil {
		if errors.Is(err, errors.ErrCustomerNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Shown once to the operator and never stored in the clear — only the hash
	// is written, so there is no plaintext copy anywhere afterwards. The same
	// value is not emailed: an operator cannot verify who is at the other end
	// of an address, and a password that leaves the building in a message
	// outlives any decision to revoke it.
	password := security.RandomString()
	hash, err := security.HashPassword(password)
	if err != nil {
		// Storing what bcrypt refused would leave an account no password can
		// open, and the operator would learn about it from the buyer.
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if err := db.SetCustomerPassword(c.Context(), customerID, hash); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// A reset is what an operator reaches for when they suspect the account is
	// no longer only the buyer's, so the sessions that suspicion covers have to
	// go with the old password.
	if err := db.RevokeCustomerSessions(c.Context(), customerID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Customer password updated", map[string]any{
		"id":       customerID,
		"password": password,
	})
}

// DeleteCustomer removes a customer account.
//
// @Summary      Delete customer
// @Description  Delete a customer account and end its sessions; the carts stay as the record of what was paid
// @Tags         Customers
// @Security     BearerAuth
// @Produce      json
// @Param        customer_id path string true "Customer ID"
// @Success      200 {object} webutil.HTTPResponse "Customer deleted"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/_/customers/{customer_id} [delete]
func DeleteCustomer(c fiber.Ctx) error {
	customerID := c.Params("customer_id")
	db := queries.DB()
	log := logging.New()

	// Sessions first, then the account: if the second step fails the customer
	// is merely signed out, which a retry fixes, while the other order would
	// leave a worked session behind a deleted account.
	if err := db.RevokeCustomerSessions(c.Context(), customerID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	if err := db.DeleteCustomer(c.Context(), customerID); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Customer deleted", nil)
}

// shopCurrency reads the currency the shop sells in.
//
// The customer list totals what each address has spent, and a cart keeps the
// currency it was created in: a shop that changed currency has a history in two
// of them, and a figure that added those minor units together would be wrong
// without looking wrong. The totals are therefore computed in this currency and
// labelled with it.
func shopCurrency(c fiber.Ctx) (string, error) {
	setting, err := queries.DB().GetSettingByKey(c.Context(), "currency")
	if err != nil {
		return "", err
	}

	currency, ok := setting["currency"].Value.(string)
	if !ok || currency == "" {
		// Answered rather than defaulted: every total in the list is summed in
		// this currency, and a list labelled with an empty one would look like
		// a shop that trades in nothing.
		return "", errors.ErrSettingNotFound
	}

	return currency, nil
}

// parseBoolQuery reads a boolean query parameter, treating anything it cannot
// parse as absent rather than as an error: a filter is a convenience, and a
// malformed one should widen the list, not fail the page.
func parseBoolQuery(c fiber.Ctx, key string) bool {
	value, err := strconv.ParseBool(c.Query(key))
	return err == nil && value
}
