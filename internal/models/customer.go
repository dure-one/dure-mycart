package models

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

// Customer is a storefront account: the buyer who signs in to the cabinet to
// reach what they purchased.
//
// It is a separate identity from the admin account, which lives in the setting
// table and is a single, singular user. Customer rows are many, and a customer
// session must never be accepted by the admin API — the two surfaces are kept
// apart by the signing key, the cookie name and the role recorded in the
// session row (see internal/middleware).
type Customer struct {
	Core
	Email    string `json:"email"`
	Password string `json:"-"`
	Name     string `json:"name,omitempty"`
	Active   bool   `json:"active"`
}

// CustomerSignUp is the registration payload.
//
// The password is bounded at 72 bytes because that is where bcrypt stops
// reading: accepting more would silently ignore the tail and let two different
// passwords open the same account.
type CustomerSignUp struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

// Validate is ...
func (v CustomerSignUp) Validate() error {
	return validation.ValidateStruct(&v,
		validation.Field(&v.Email, validation.Required, is.Email),
		validation.Field(&v.Password, validation.Required, validation.Length(8, 72)),
		validation.Field(&v.Name, validation.Length(0, 100)),
	)
}

// CustomerSummary is one row of the admin customer list.
//
// A row is keyed by email rather than by account, because in this shop most
// buyers never register: they check out as guests, and the address on the cart
// is the only thing that identifies them. So the list is the union of the
// accounts in the customer table and the addresses that have ever paid, and a
// row carries Registered=false when only the latter matched.
//
// ID, Created, Updated and Active are zero for such a row: there is no account
// behind it, so no account-level action (block, reset password, delete) applies
// and the flag says nothing about the buyer. Callers must read Registered
// before they read Active.
type CustomerSummary struct {
	Core
	Email      string `json:"email"`
	Name       string `json:"name,omitempty"`
	Active     bool   `json:"active"`
	Registered bool   `json:"registered"`
	Purchases  int    `json:"purchases"`
	Spent      int    `json:"spent"`
	Currency   string `json:"currency"`
	LastOrder  int64  `json:"last_order,omitempty"`
}

// CustomerPurchase is one paid order as the cabinet presents it.
//
// The address the buyer typed at checkout is deliberately absent. The cabinet
// only ever shows the orders of the signed-in customer, so the field would hold
// the same string on every row — and a response that names its own subject is
// one careless copy-paste away from answering about somebody else.
type CustomerPurchase struct {
	ID          string                 `json:"id"`
	Created     int64                  `json:"created"`
	AmountTotal int                    `json:"amount_total"`
	Currency    string                 `json:"currency"`
	Items       []CustomerPurchaseItem `json:"items"`
}

// CustomerPurchaseItem is one line of a purchase, with what the buyer may take
// away from it.
type CustomerPurchaseItem struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Quantity  int    `json:"quantity"`
	// Digital is the delivery kind of the product: "file", "data", "api", or
	// empty for a product that hands nothing over.
	Digital string `json:"digital,omitempty"`
	// Files are the product's downloadable files, filled when Digital is
	// "file". Every buyer of the product is offered the same files — what was
	// sold is the product, not a copy of it — which is why the download
	// endpoint checks the entitlement again on each request instead of treating
	// possession of a file id as proof of purchase.
	Files []CustomerPurchaseFile `json:"files,omitempty"`
	// Codes are the license keys this order claimed, filled when Digital is
	// "data". Only the keys this order claimed: a product holds one row per
	// key, and the unclaimed ones belong to nobody yet.
	Codes []string `json:"codes,omitempty"`
}

// CustomerPurchaseFile is a downloadable file of a purchased product.
//
// ID is what the download endpoint takes; OrigName is the name the operator
// uploaded it under, which is what the buyer will find on disk.
type CustomerPurchaseFile struct {
	ID       string `json:"id"`
	OrigName string `json:"orig_name"`
}
