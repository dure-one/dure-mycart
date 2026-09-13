package queries

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/litepay"
	"github.com/shurco/mycart/pkg/security"
)

// DefaultAccountExpireHours is how long a cabinet session lasts when the
// operator leaves account_jwt_expire_hours at zero. Thirty days: long enough
// that a buyer is not asked to sign in again on every visit, short enough that
// an abandoned session on a shared device does not live forever.
const DefaultAccountExpireHours = 720

// accountSecretKey is the setting holding the storefront signing key.
const accountSecretKey = "account_jwt_secret"

// CustomerQueries is a struct that holds a dialect-aware database handle for
// the storefront customer accounts.
type CustomerQueries struct {
	DB *database.Conn
}

// NormalizeEmail trims and lower-cases an address so the same person cannot end
// up with two accounts, and so the lookup at sign-in matches what sign-up
// stored regardless of how the address was typed either time.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// customerExists reports whether an address already has an account. It is the
// uniqueness check of CreateCustomer, split out so the same question can be
// asked again after a losing race on the insert.
func (q *CustomerQueries) customerExists(ctx context.Context, email string) (bool, error) {
	var taken int
	if err := q.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM customer WHERE email = ?`, email).Scan(&taken); err != nil {
		return false, err
	}
	return taken > 0, nil
}

// CreateCustomer stores a new account with the password already hashed.
//
// The uniqueness check is a separate SELECT rather than reliance on the UNIQUE
// constraint, because the two engines report a violation differently and the
// caller needs to tell a taken address from a broken database. The constraint
// still backs it up: two simultaneous sign-ups for one address race past the
// SELECT, and the loser is told the address is taken rather than that the shop
// is broken.
func (q *CustomerQueries) CreateCustomer(ctx context.Context, email, password, name string) (*models.Customer, error) {
	email = NormalizeEmail(email)

	taken, err := q.customerExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errors.ErrCustomerEmailTaken
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}

	customer := &models.Customer{
		Core:     models.Core{ID: security.RandomString()},
		Email:    email,
		Password: hash,
		Name:     strings.TrimSpace(name),
		Active:   true,
	}

	query := fmt.Sprintf(
		`INSERT INTO customer (id, email, password, name, active) VALUES (?, ?, ?, ?, ?) RETURNING %s`,
		q.DB.Dialect().Epoch("created"))

	if err := q.DB.QueryRowContext(ctx, query,
		customer.ID, customer.Email, customer.Password, customer.Name, customer.Active,
	).Scan(&customer.Created); err != nil {
		// The loser of a race between two sign-ups fails here, on the UNIQUE
		// constraint. The engines spell that violation differently and the
		// query layer deliberately knows nothing about either, so the address
		// is looked up again instead: if it exists now, the caller is told what
		// a retry can act on.
		if taken, lookupErr := q.customerExists(ctx, email); lookupErr == nil && taken {
			return nil, errors.ErrCustomerEmailTaken
		}
		return nil, err
	}

	return customer, nil
}

// CustomerByEmail returns the account with the given address, or
// errors.ErrCustomerNotFound.
func (q *CustomerQueries) CustomerByEmail(ctx context.Context, email string) (*models.Customer, error) {
	return q.customer(ctx, "WHERE email = ?", NormalizeEmail(email))
}

// CustomerByID returns the account with the given id, or
// errors.ErrCustomerNotFound.
func (q *CustomerQueries) CustomerByID(ctx context.Context, id string) (*models.Customer, error) {
	return q.customer(ctx, "WHERE id = ?", id)
}

// customer runs a single-row lookup and turns "no rows" into the package's
// sentinel, so handlers branch with errors.Is instead of the driver's error.
//
// The timestamps come back as unix seconds like everywhere else, and are read
// through nullable holders: this is also the lookup the cabinet middleware
// makes on every request, so it stays a single indexed row.
func (q *CustomerQueries) customer(ctx context.Context, where string, arg any) (*models.Customer, error) {
	customer := &models.Customer{}
	var created, updated sql.NullInt64

	query := fmt.Sprintf(
		`SELECT id, email, password, name, active, %s, %s FROM customer %s`,
		q.DB.Dialect().Epoch("created"), q.DB.Dialect().Epoch("updated"), where)

	err := q.DB.QueryRowContext(ctx, query, arg).Scan(
		&customer.ID, &customer.Email, &customer.Password, &customer.Name, &customer.Active,
		&created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrCustomerNotFound
		}
		return nil, err
	}

	customer.Created = created.Int64
	customer.Updated = updated.Int64

	return customer, nil
}

// Account returns the storefront account settings.
func (q *CustomerQueries) Account(ctx context.Context) (*models.Account, error) {
	setting, err := NewBase(q.DB).GetSettingByGroup(ctx, new(models.Account))
	if err != nil {
		return nil, err
	}
	return setting.(*models.Account), nil
}

// AccountEnabled reports whether the customer cabinet is available. It is FALSE
// on a fresh installation.
func (q *CustomerQueries) AccountEnabled(ctx context.Context) (bool, error) {
	account, err := q.Account(ctx)
	if err != nil {
		return false, err
	}
	return account.Enabled, nil
}

// AccountSecret returns the key that signs storefront session tokens, creating
// it on first use.
//
// It is read and written only from here: the key is not a field of
// models.Account, so it is neither returned to the admin API nor rewritten when
// an operator saves the cabinet settings.
//
// It must not be the admin jwt_secret. Both keys sign tokens whose only claim
// is a session id, and both middlewares accept a token on the strength of a
// row in the shared session table — so a single key, plus one storefront
// session, would be enough to sign a token the admin API would honour. The
// secret is generated here rather than seeded by a migration because a key
// written into a public SQL file is a published key.
func (q *CustomerQueries) AccountSecret(ctx context.Context) (string, error) {
	var value string
	if err := q.DB.QueryRowContext(ctx,
		`SELECT value FROM setting WHERE key = ?`, accountSecretKey).Scan(&value); err != nil {
		return "", err
	}
	if value != "" {
		return value, nil
	}

	token, err := security.NewToken(security.RandomString())
	if err != nil {
		return "", err
	}

	// Fill the empty row, then re-read it. Two requests that find the secret
	// missing at the same moment must end up agreeing on one key: the loser of
	// this race returns what was stored, not what it generated, or it would
	// hand out tokens that nothing can verify.
	if _, err := q.DB.ExecContext(ctx,
		`UPDATE setting SET value = ? WHERE key = ? AND value = ''`, token, accountSecretKey); err != nil {
		return "", err
	}

	if err := q.DB.QueryRowContext(ctx,
		`SELECT value FROM setting WHERE key = ?`, accountSecretKey).Scan(&value); err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.ErrSettingNotFound
	}

	return value, nil
}

// CustomerFilter narrows the admin customer list.
type CustomerFilter struct {
	// Search matches a substring of the address or the name, case-insensitively.
	Search string
	// RegisteredOnly drops the buyers who never created an account, leaving
	// only the rows an account-level action can be applied to.
	RegisteredOnly bool
	// Currency is the currency the spent totals are computed in. Carts keep the
	// currency they were created in, so a shop that changed currency has a
	// history in two of them; adding those minor units together would be
	// arithmetic on nothing. Totals therefore cover the given currency only,
	// and the count of purchases covers everything.
	Currency string
}

// likeContains renders a substring pattern for LIKE with the wildcards escaped,
// so that searching for "100%" looks for those four characters instead of
// matching every row. The callers pair it with `LIKE ? ESCAPE '\'`.
func likeContains(s string) string {
	escape := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + escape.Replace(strings.ToLower(s)) + "%"
}

// Customers returns one row per address the shop knows about, newest activity
// first.
//
// A "customer" here is an address, not an account: most buyers in this shop
// check out as guests, so the list is the union of the registered accounts and
// the addresses that have paid at least once. A row from the second group has
// no account behind it, which the caller can tell from the summary's
// Registered flag.
//
// Addresses are compared trimmed and lower-cased throughout. Checkout stores
// the address exactly as the buyer typed it, while registration normalises it,
// so "Anna@Example.com " and "anna@example.com" have to collapse into one row —
// the alternative is an operator seeing the same person twice and blocking the
// half that is not actually used to sign in. The same trimming is what keeps a
// cart whose address was typed as blanks out of the list: it collapses to "" and
// the guard drops it rather than showing a row with no address at all.
func (q *CustomerQueries) Customers(
	ctx context.Context, filter CustomerFilter, limit, offset int,
) ([]*models.CustomerSummary, int, error) {
	epochCreated := q.DB.Dialect().Epoch("c.created")
	epochUpdated := q.DB.Dialect().Epoch("c.updated")

	// One row per address, with the account (if any) hanging off it.
	addresses := `
	FROM (
		SELECT email FROM customer
		UNION
		SELECT LOWER(TRIM(email)) AS email FROM cart WHERE TRIM(email) <> '' AND payment_status = ?
	) e
	LEFT JOIN customer c ON c.email = e.email
	`

	var conditions []string
	// filterParams holds the search patterns alone, so the two callers below
	// can put them after their own parameters.
	var filterParams []any
	if filter.Search != "" {
		conditions = append(conditions,
			`(e.email LIKE ? ESCAPE '\' OR LOWER(COALESCE(c.name, '')) LIKE ? ESCAPE '\')`)
		pattern := likeContains(filter.Search)
		filterParams = append(filterParams, pattern, pattern)
	}
	if filter.RegisteredOnly {
		conditions = append(conditions, "c.id IS NOT NULL")
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// The count reads the addresses and their accounts only. Joining the
	// purchase totals here would make it GROUP BY the whole cart table to
	// produce a number it never looks at.
	addressParams := append([]any{litepay.PAID}, filterParams...)
	var total int
	if err := q.DB.QueryRowContext(ctx,
		"SELECT COUNT(*)"+addresses+where, addressParams...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// One row per address again, so this join cannot multiply the one above.
	totals := fmt.Sprintf(`
	LEFT JOIN (
		SELECT
			LOWER(TRIM(email)) AS email,
			COUNT(*) AS purchases,
			SUM(CASE WHEN currency = ? THEN amount_total ELSE 0 END) AS spent,
			MAX(%s) AS last_order
		FROM cart
		WHERE payment_status = ? AND TRIM(email) <> ''
		GROUP BY LOWER(TRIM(email))
	) o ON o.email = e.email
	`, q.DB.Dialect().Epoch("created"))

	// Only the column list goes through Sprintf. The FROM clause is
	// concatenated onto it rather than interpolated into the format string,
	// because the epoch fragments it carries are themselves of the form
	// strftime('%s', …): a percent sign that reaches a format string is not a
	// syntax error, it silently consumes the next argument and leaves the
	// query quietly answering a different question.
	//
	// account_id is the raw join key, deliberately not coalesced: NULL is how a
	// row says "this address never registered", which is the one thing the
	// caller has to know before offering an account-level action.
	columns := fmt.Sprintf(`
	SELECT
		c.id AS account_id,
		e.email,
		COALESCE(c.name, '') AS name,
		c.active,
		%s AS created,
		%s AS updated,
		COALESCE(o.purchases, 0) AS purchases,
		COALESCE(o.spent, 0) AS spent,
		COALESCE(o.last_order, 0) AS last_order
	`, epochCreated, epochUpdated)

	// Buyers first, then addresses that only ever registered, and within each
	// group the most recent purchase first. The leading (… IS NULL) sort key is
	// what keeps the two engines' opposite NULL placement out of the result:
	// ordering on the timestamp alone would put one group at the wrong end on
	// one of them.
	order := "\n\tORDER BY (o.last_order IS NULL), o.last_order DESC, e.email\n"

	query := columns + addresses + totals + where + order + " LIMIT ? OFFSET ?"

	// The order of these parameters is the textual order of the placeholders:
	// the paid guard inside the address union, then the two in the totals join,
	// then the search patterns of the WHERE clause, then pagination.
	rowParams := append([]any{litepay.PAID, filter.Currency, litepay.PAID}, filterParams...)
	rowParams = append(rowParams, limit, offset)

	rows, err := q.DB.QueryContext(ctx, query, rowParams...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	customers := []*models.CustomerSummary{}
	for rows.Next() {
		var accountID, name sql.NullString
		var active sql.NullBool
		var created, updated, lastOrder sql.NullInt64
		summary := &models.CustomerSummary{}

		if err := rows.Scan(
			&accountID,
			&summary.Email,
			&name,
			&active,
			&created,
			&updated,
			&summary.Purchases,
			&summary.Spent,
			&lastOrder,
		); err != nil {
			return nil, 0, err
		}

		summary.ID = accountID.String
		summary.Registered = accountID.Valid
		summary.Name = name.String
		summary.Active = active.Bool
		summary.Created = created.Int64
		summary.Updated = updated.Int64
		summary.LastOrder = lastOrder.Int64
		// The totals are in this currency by construction — the query only adds
		// up carts that carry it — so the row has to say which one that was, or
		// the operator is looking at a number with no unit.
		summary.Currency = filter.Currency

		customers = append(customers, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

// SetCustomerActive blocks or unblocks an account.
func (q *CustomerQueries) SetCustomerActive(ctx context.Context, id string, active bool) error {
	_, err := q.DB.ExecContext(ctx,
		`UPDATE customer SET active = ?, updated = CURRENT_TIMESTAMP WHERE id = ?`, active, id)
	return err
}

// SetCustomerPassword replaces the stored password hash.
//
// The hash is stored already hashed: this function is deliberately unable to
// take a plaintext password, so no caller can leave one lying in a column.
func (q *CustomerQueries) SetCustomerPassword(ctx context.Context, id, passwordHash string) error {
	_, err := q.DB.ExecContext(ctx,
		`UPDATE customer SET password = ?, updated = CURRENT_TIMESTAMP WHERE id = ?`, passwordHash, id)
	return err
}

// DeleteCustomer removes an account.
//
// The carts are left in place. They are the record of a payment that happened,
// and an operator deleting an account (a buyer asking to be forgotten, a
// duplicate created by a typo) must not thereby erase the shop's takings.
func (q *CustomerQueries) DeleteCustomer(ctx context.Context, id string) error {
	_, err := q.DB.ExecContext(ctx, `DELETE FROM customer WHERE id = ?`, id)
	return err
}

// RevokeCustomerSessions drops every cabinet session belonging to a customer.
//
// The session table is the revocation list for the storefront: a token is
// honoured only while its row exists, and the row's value carries the customer
// id, so deleting by value ends the session on every device the buyer signed in
// from rather than on the one the operator happens to know about.
func (q *CustomerQueries) RevokeCustomerSessions(ctx context.Context, customerID string) error {
	_, err := q.DB.ExecContext(ctx,
		`DELETE FROM session WHERE value = ?`, SessionValue(SessionRoleCustomer, customerID))
	return err
}

// CustomerPurchases returns the paid orders of an address, newest first, each
// with its lines and the digital goods those lines hand over.
//
// Only paid orders: a cart that was never paid for is not a purchase, and this
// is the list the shop calls the buyer's own. The address is compared
// normalized, because checkout keeps whatever was typed while the account
// stores the lower-cased form — otherwise a buyer who typed a capital letter
// would find an empty cabinet.
func (q *CustomerQueries) CustomerPurchases(ctx context.Context, email string) ([]*models.CustomerPurchase, error) {
	carts, err := NewBase(q.DB).PaidCartsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	purchases := []*models.CustomerPurchase{}
	for _, cart := range carts {
		items, err := q.purchaseItems(ctx, cart)
		if err != nil {
			return nil, err
		}

		purchases = append(purchases, &models.CustomerPurchase{
			ID:          cart.ID,
			Created:     cart.Created,
			AmountTotal: cart.AmountTotal,
			Currency:    cart.Currency,
			Items:       items,
		})
	}

	return purchases, nil
}

// CustomerOwnsProduct reports whether an address has ever paid for a product.
//
// The lines of each cart are parsed here rather than matched in SQL. A cart's
// contents live in a text column holding JSON, and the only comparison SQL
// offers over it is a substring one — a match on a string, not on the id field,
// which is both weaker than it looks and spelled differently on the two
// engines. A buyer has a handful of orders, so reading them costs nothing and
// answers the question exactly.
func (q *CustomerQueries) CustomerOwnsProduct(ctx context.Context, email, productID string) (bool, error) {
	carts, err := NewBase(q.DB).PaidCartsByEmail(ctx, email)
	if err != nil {
		return false, err
	}

	for _, cart := range carts {
		for _, line := range cart.Cart {
			if line.ProductID == productID {
				return true, nil
			}
		}
	}

	return false, nil
}

// EntitledDigitalFile returns a digital file if the address has paid for the
// product it belongs to, and errors.ErrProductNotFound otherwise.
//
// The one error answers "no such file" and "that one is not yours" alike, and
// that is the point: told apart, the endpoint becomes a way to walk the shop's
// catalogue of guides by id and learn what exists.
func (q *CustomerQueries) EntitledDigitalFile(ctx context.Context, email, fileID string) (*models.File, error) {
	file, productID, err := q.digitalFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	owned, err := q.CustomerOwnsProduct(ctx, email, productID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.ErrProductNotFound
	}

	return file, nil
}

// purchaseItems resolves the lines of one order.
//
// A line whose product no longer exists is dropped rather than shown blank: the
// cart stores ids only, so once the row is gone there is no name left to
// display, and a nameless line with nothing behind it is worse than an order
// that reads one item shorter. The order itself stays, with its total.
func (q *CustomerQueries) purchaseItems(ctx context.Context, cart *models.Cart) ([]models.CustomerPurchaseItem, error) {
	items := []models.CustomerPurchaseItem{}

	for _, line := range cart.Cart {
		product, err := q.purchaseProduct(ctx, line.ProductID)
		if err != nil {
			if errors.Is(err, errors.ErrProductNotFound) {
				continue
			}
			return nil, err
		}

		item := models.CustomerPurchaseItem{
			ProductID: product.ID,
			Name:      product.Name,
			Slug:      product.Slug,
			Quantity:  line.Quantity,
			Digital:   product.Digital.Type,
		}

		switch product.Digital.Type {
		case "file":
			if item.Files, err = q.purchaseFiles(ctx, product.ID); err != nil {
				return nil, err
			}
		case "data":
			if item.Codes, err = q.purchaseCodes(ctx, cart.ID, product.ID); err != nil {
				return nil, err
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// purchaseProduct reads the fields of a product that a list of orders shows.
//
// Deliberately neither of the two lookups the rest of the shop uses. The public
// one hides anything the operator has not published, and a guide taken off sale
// is still owed to the people who paid for it. The admin one loads variants,
// options, metadata and images, none of which an order list shows and all of
// which the buyer is the wrong caller to hand them to.
func (q *CustomerQueries) purchaseProduct(ctx context.Context, productID string) (*models.Product, error) {
	var digital sql.NullString
	product := &models.Product{}

	err := q.DB.QueryRowContext(ctx,
		`SELECT id, name, slug, digital FROM product WHERE id = ?`, productID).
		Scan(&product.ID, &product.Name, &product.Slug, &digital)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrProductNotFound
		}
		return nil, err
	}

	product.Digital.Type = digital.String

	return product, nil
}

// purchaseFiles lists the downloadable files of a purchased product.
func (q *CustomerQueries) purchaseFiles(ctx context.Context, productID string) ([]models.CustomerPurchaseFile, error) {
	// Ordered by the name the buyer will see rather than by the stored one: the
	// two differ only for a file stored without an upload name, and sorting on
	// the empty string would put exactly that file first.
	rows, err := q.DB.QueryContext(ctx,
		`SELECT id, name, ext, orig_name FROM digital_file WHERE product_id = ?
			ORDER BY COALESCE(NULLIF(orig_name, ''), name || '.' || ext), id`,
		productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	files := []models.CustomerPurchaseFile{}
	for rows.Next() {
		var id, name, ext, origName string
		if err := rows.Scan(&id, &name, &ext, &origName); err != nil {
			return nil, err
		}

		// A row stored without a name would reach the buyer as a blank label on
		// a link; the stored name and extension are always there and make a
		// name of sorts. Same fallback the download endpoint puts into the
		// Content-Disposition header.
		if origName == "" {
			origName = name + "." + ext
		}

		files = append(files, models.CustomerPurchaseFile{ID: id, OrigName: origName})
	}

	return files, rows.Err()
}

// purchaseCodes returns the license keys an order claimed for a product.
//
// Scoped to the cart as well as the product, and that scoping is the whole
// entitlement: a product holds one row per key, most of them unclaimed and
// waiting for a buyer, so a lookup by product alone would print the shop's
// entire remaining stock into the first cabinet that asked.
func (q *CustomerQueries) purchaseCodes(ctx context.Context, cartID, productID string) ([]string, error) {
	rows, err := q.DB.QueryContext(ctx,
		`SELECT content FROM digital_data WHERE cart_id = ? AND product_id = ? ORDER BY id`,
		cartID, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	codes := []string{}
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, err
		}
		codes = append(codes, content)
	}

	return codes, rows.Err()
}

// digitalFile reads one digital file and the product it belongs to.
func (q *CustomerQueries) digitalFile(ctx context.Context, fileID string) (*models.File, string, error) {
	file := &models.File{}
	var productID string

	err := q.DB.QueryRowContext(ctx,
		`SELECT id, name, ext, orig_name, product_id FROM digital_file WHERE id = ?`, fileID).
		Scan(&file.ID, &file.Name, &file.Ext, &file.OrigName, &productID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", errors.ErrProductNotFound
		}
		return nil, "", err
	}

	if file.OrigName == "" {
		file.OrigName = file.Name + "." + file.Ext
	}

	return file, productID, nil
}
