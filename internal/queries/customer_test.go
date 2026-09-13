package queries

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/litepay"
)

func TestCustomerLifecycle(t *testing.T) {
	db, ctx := bootstrap(t)

	customer, err := db.CreateCustomer(ctx, "  Buyer@Example.COM ", "Passw0rd!", " Ada ")
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}
	if customer.ID == "" {
		t.Error("expected an id")
	}
	// The address is stored normalised, otherwise the same person could hold
	// two accounts and the lookup at sign-in would miss the one sign-up wrote.
	if customer.Email != "buyer@example.com" {
		t.Errorf("email = %q, want buyer@example.com", customer.Email)
	}
	if customer.Name != "Ada" {
		t.Errorf("name = %q, want Ada", customer.Name)
	}
	if !customer.Active {
		t.Error("a new account must be active")
	}
	// The password must never be stored as given.
	if strings.Contains(customer.Password, "Passw0rd!") || customer.Password == "" {
		t.Errorf("password is not hashed: %q", customer.Password)
	}

	// Lookups normalise their argument too, so the case the buyer typed at
	// sign-in does not have to match the one they typed at sign-up.
	byEmail, err := db.CustomerByEmail(ctx, "BUYER@example.com")
	if err != nil {
		t.Fatalf("CustomerByEmail: %v", err)
	}
	if byEmail.ID != customer.ID {
		t.Errorf("byEmail id = %q, want %q", byEmail.ID, customer.ID)
	}

	byID, err := db.CustomerByID(ctx, customer.ID)
	if err != nil {
		t.Fatalf("CustomerByID: %v", err)
	}
	if byID.Email != customer.Email {
		t.Errorf("byID email = %q, want %q", byID.Email, customer.Email)
	}

	// A second account for the same address is refused rather than silently
	// creating a duplicate.
	if _, err := db.CreateCustomer(ctx, "buyer@example.com", "Other1!x", ""); !errors.Is(err, errors.ErrCustomerEmailTaken) {
		t.Errorf("duplicate signup: got %v, want ErrCustomerEmailTaken", err)
	}

	if _, err := db.CustomerByEmail(ctx, "nobody@example.com"); !errors.Is(err, errors.ErrCustomerNotFound) {
		t.Errorf("unknown email: got %v, want ErrCustomerNotFound", err)
	}
	if _, err := db.CustomerByID(ctx, "missing"); !errors.Is(err, errors.ErrCustomerNotFound) {
		t.Errorf("unknown id: got %v, want ErrCustomerNotFound", err)
	}
}

func TestAccountDisabledByDefault(t *testing.T) {
	db, ctx := bootstrap(t)

	// A fresh installation must not present a cabinet: the migration seeds the
	// switch off, and nothing here may turn it on by accident.
	enabled, err := db.AccountEnabled(ctx)
	if err != nil {
		t.Fatalf("AccountEnabled: %v", err)
	}
	if enabled {
		t.Error("the cabinet must be off on a fresh installation")
	}

	if err := db.UpdateSettingByGroup(ctx, &models.Account{Enabled: true, ExpireHours: 24}); err != nil {
		t.Fatalf("UpdateSettingByGroup: %v", err)
	}

	enabled, err = db.AccountEnabled(ctx)
	if err != nil {
		t.Fatalf("AccountEnabled after enable: %v", err)
	}
	if !enabled {
		t.Error("the cabinet must be on after enabling it")
	}

	account, err := db.Account(ctx)
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
	if account.ExpireHours != 24 {
		t.Errorf("expire hours = %d, want 24", account.ExpireHours)
	}
}

func TestAccountSecret(t *testing.T) {
	db, ctx := bootstrap(t)

	// The migration seeds the key empty, so the first caller has to generate
	// one: nothing may ever be signed with an empty secret.
	first, err := db.AccountSecret(ctx)
	if err != nil {
		t.Fatalf("AccountSecret: %v", err)
	}
	if len(first) < 32 {
		t.Errorf("secret is %d characters, too short to be a signing key", len(first))
	}

	second, err := db.AccountSecret(ctx)
	if err != nil {
		t.Fatalf("AccountSecret second call: %v", err)
	}
	if second != first {
		t.Error("the secret must be generated once and then reused, or every session would be invalidated")
	}

	// It must be a key of its own. Sharing the admin secret would let a token
	// minted for a customer verify against the admin API, because both tokens
	// carry nothing but a session id.
	settingJWT, err := GetSettingByGroup[models.JWT](ctx, db)
	if err != nil {
		t.Fatalf("GetSettingByGroup[JWT]: %v", err)
	}
	if settingJWT.Secret == first {
		t.Error("the cabinet secret must not be the admin jwt secret")
	}
}

// TestCustomers covers the admin list. It is the one query here that reads two
// sources at once, so it is where a mistake shows up as a row the operator
// cannot account for: the same buyer twice under two spellings, a guest they
// cannot reach, or a total that added two currencies together.
func TestCustomers(t *testing.T) {
	db, ctx := bootstrap(t)

	if _, err := db.CreateCustomer(ctx, "anna@example.com", "Passw0rd!", "Fern"); err != nil {
		t.Fatalf("create anna: %v", err)
	}
	// Registered and never bought: the list is every address the shop knows, not
	// only the ones that paid.
	if _, err := db.CreateCustomer(ctx, "zoe@example.com", "Passw0rd!", ""); err != nil {
		t.Fatalf("create zoe: %v", err)
	}

	// A guest with two orders, typed here the way checkout keeps it: one in the
	// currency the shop sells in, one from before it changed.
	addPaidCart(t, db, ctx, "list-guest-eur", "  Guest@Example.COM  ", litepay.PAID, "2024-07-01 10:00:00",
		models.CartProduct{ProductID: "guide", Quantity: 2})
	usd := addPaidCart(t, db, ctx, "list-guest-usd", "guest@example.com", litepay.PAID, "2024-07-02 10:00:00",
		models.CartProduct{ProductID: "guide", Quantity: 1})
	if _, err := Conn().ExecContext(ctx, `UPDATE cart SET currency = ? WHERE id = ?`, "USD", usd.ID); err != nil {
		t.Fatalf("price a cart in dollars: %v", err)
	}

	// The account's one order, older than the guest's newest and spelled
	// differently, which must not put the buyer in the list twice.
	addPaidCart(t, db, ctx, "list-anna", "ANNA@example.com", litepay.PAID, "2024-06-01 10:00:00",
		models.CartProduct{ProductID: "guide", Quantity: 1})

	// Neither of these is a customer: an abandoned cart is not a buyer, and a
	// cart whose address is blanks has no address to be listed under.
	addPaidCart(t, db, ctx, "list-unpaid", "abandoned@example.com", litepay.UNPAID, "2024-08-01 10:00:00",
		models.CartProduct{ProductID: "guide", Quantity: 1})
	addPaidCart(t, db, ctx, "list-blank", "   ", litepay.PAID, "2024-08-02 10:00:00",
		models.CartProduct{ProductID: "guide", Quantity: 1})

	rows, total, err := db.Customers(ctx, CustomerFilter{Currency: "EUR"}, 10, 0)
	if err != nil {
		t.Fatalf("Customers: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("total = %d, rows = %d, want 3 and 3: %+v", total, len(rows), rows)
	}

	// Buyers first, newest order first, then the addresses that have only ever
	// registered — something the two engines disagree about unless the sort key
	// says so explicitly.
	if rows[0].Email != "guest@example.com" || rows[1].Email != "anna@example.com" || rows[2].Email != "zoe@example.com" {
		t.Fatalf("order = [%s %s %s], want guest, anna, zoe", rows[0].Email, rows[1].Email, rows[2].Email)
	}
	if rows[0].LastOrder <= rows[1].LastOrder {
		t.Errorf("last order = %d against %d, want newest first", rows[0].LastOrder, rows[1].LastOrder)
	}

	guest := rows[0]
	if guest.Registered || guest.ID != "" {
		t.Errorf("the guest must be listed without an account: %+v", guest)
	}
	// Both orders count, but only the euro one is added up: a dollar figure
	// summed into a euro total would be wrong without looking wrong.
	if guest.Purchases != 2 || guest.Spent != 5000 {
		t.Errorf("guest = %d purchases, %d spent, want 2 and 5000", guest.Purchases, guest.Spent)
	}
	if guest.Currency != "EUR" {
		t.Errorf("currency = %q, want the currency the totals are in", guest.Currency)
	}

	anna := rows[1]
	if !anna.Registered || anna.ID == "" || anna.Name != "Fern" {
		t.Errorf("anna must carry her account: %+v", anna)
	}
	if anna.Purchases != 1 || anna.Spent != 2500 {
		t.Errorf("anna = %d purchases, %d spent, want 1 and 2500", anna.Purchases, anna.Spent)
	}

	zoe := rows[2]
	if !zoe.Registered || zoe.Purchases != 0 || zoe.Spent != 0 || zoe.LastOrder != 0 {
		t.Errorf("zoe has no orders and no totals: %+v", zoe)
	}

	// The totals are computed in the requested currency, so asking for dollars
	// must hand back the dollar order rather than the euro one.
	usdRows, _, err := db.Customers(ctx, CustomerFilter{Currency: "USD"}, 10, 0)
	if err != nil {
		t.Fatalf("Customers in USD: %v", err)
	}
	if usdRows[0].Spent != 2500 || usdRows[0].Purchases != 2 {
		t.Errorf("usd guest = %d spent over %d purchases, want 2500 over 2", usdRows[0].Spent, usdRows[0].Purchases)
	}

	// The count is what the pager needs, so it has to cover the whole result and
	// not the page that happens to be asked for.
	page, total, err := db.Customers(ctx, CustomerFilter{Currency: "EUR"}, 1, 0)
	if err != nil {
		t.Fatalf("Customers page: %v", err)
	}
	if len(page) != 1 || total != 3 {
		t.Errorf("page = %d rows of %d, want 1 of 3", len(page), total)
	}

	registered, total, err := db.Customers(ctx, CustomerFilter{Currency: "EUR", RegisteredOnly: true}, 10, 0)
	if err != nil {
		t.Fatalf("Customers registered: %v", err)
	}
	if total != 2 || len(registered) != 2 || registered[0].Email != "anna@example.com" {
		t.Errorf("registered only = %d of %d: %+v", len(registered), total, registered)
	}

	// Search reaches both halves of a row: the address, and the name behind it.
	byName, total, err := db.Customers(ctx, CustomerFilter{Currency: "EUR", Search: "fern"}, 10, 0)
	if err != nil {
		t.Fatalf("Customers by name: %v", err)
	}
	if total != 1 || len(byName) != 1 || byName[0].Email != "anna@example.com" {
		t.Errorf("search by name = %+v", byName)
	}

	byEmail, total, err := db.Customers(ctx, CustomerFilter{Currency: "EUR", Search: "ZOE"}, 10, 0)
	if err != nil {
		t.Fatalf("Customers by email: %v", err)
	}
	if total != 1 || len(byEmail) != 1 || byEmail[0].Email != "zoe@example.com" {
		t.Errorf("search by email = %+v", byEmail)
	}

	// A search term is text the operator typed, not a pattern. Wildcards in it
	// have to look for themselves — otherwise a single "%" lists the whole shop
	// and, worse, reads as if it had matched something.
	for _, term := range []string{"%", "_"} {
		none, total, err := db.Customers(ctx, CustomerFilter{Currency: "EUR", Search: term}, 10, 0)
		if err != nil {
			t.Fatalf("Customers search %q: %v", term, err)
		}
		if total != 0 || len(none) != 0 {
			t.Errorf("search %q matched %d of %d: %+v", term, len(none), total, none)
		}
	}
}

func addProduct(t *testing.T, db *Base, ctx context.Context, id, name, digital string) *models.Product {
	t.Helper()

	product := &models.Product{
		Core:    models.Core{ID: id},
		Name:    name,
		Slug:    id,
		Amount:  2500,
		Digital: models.Digital{Type: digital},
		Active:  true,
	}
	if _, err := db.AddProductWithVariants(ctx, product); err != nil {
		t.Fatalf("add product %s: %v", id, err)
	}
	return product
}

// addPaidCart stores a cart in the given state and backdates it, so the order
// of the listing does not depend on two rows landing in the same second.
func addPaidCart(
	t *testing.T, db *Base, ctx context.Context,
	id, email string, status litepay.Status, created string, lines ...models.CartProduct,
) *models.Cart {
	t.Helper()

	cart := &models.Cart{
		Core:          models.Core{ID: id},
		Email:         email,
		Cart:          lines,
		AmountTotal:   2500 * quantityOf(lines),
		Currency:      "EUR",
		PaymentStatus: status,
		PaymentSystem: "dummy",
	}
	if err := db.AddCart(ctx, cart); err != nil {
		t.Fatalf("add cart %s: %v", id, err)
	}
	if _, err := Conn().ExecContext(ctx, `UPDATE cart SET created = ? WHERE id = ?`, created, id); err != nil {
		t.Fatalf("backdate cart %s: %v", id, err)
	}

	return cart
}

// quantityOf totals the units in a cart, so a test's expected order total reads
// as the sum of what was bought.
func quantityOf(lines []models.CartProduct) int {
	total := 0
	for _, line := range lines {
		total += line.Quantity
	}
	return total
}

// claimKey hands one license key to a cart, the way a completed purchase does.
func claimKey(t *testing.T, ctx context.Context, cartID, dataID string) {
	t.Helper()

	if _, err := Conn().ExecContext(ctx,
		`UPDATE digital_data SET cart_id = ? WHERE id = ?`, cartID, dataID); err != nil {
		t.Fatalf("claim key %s: %v", dataID, err)
	}
}

// TestCustomerPurchases covers the list the cabinet shows after sign-in: which
// orders appear, in what order, and what each line hands over.
func TestCustomerPurchases(t *testing.T) {
	db, ctx := bootstrap(t)

	guide := addProduct(t, db, ctx, "purchase-guide", "Castellon Guide", "file")
	guideFile, err := db.AddDigitalFile(ctx, guide.ID, "11111111-1111-4111-8111-111111111111", "pdf", "castellon.pdf")
	if err != nil {
		t.Fatalf("add guide file: %v", err)
	}
	// A file stored without an upload name still has to reach the buyer with a
	// label on it rather than a blank link.
	unnamed, err := db.AddDigitalFile(ctx, guide.ID, "22222222-2222-4222-8222-222222222222", "pdf", "")
	if err != nil {
		t.Fatalf("add unnamed guide file: %v", err)
	}

	keyed := addProduct(t, db, ctx, "purchase-keyed", "Valencia Guide", "data")
	claimed, err := db.AddDigitalData(ctx, keyed.ID, "KEY-THIS-ONE")
	if err != nil {
		t.Fatalf("add claimed key: %v", err)
	}
	if _, err := db.AddDigitalData(ctx, keyed.ID, "KEY-SOMEONE-ELSE"); err != nil {
		t.Fatalf("add unclaimed key: %v", err)
	}

	// The third kind of product the shop can sell: delivered by an external
	// API, so the cabinet hands over nothing for it and the line is just a
	// record that it was bought.
	external := addProduct(t, db, ctx, "purchase-external", "Paper Map", "api")

	older := addPaidCart(t, db, ctx, "purchase-older", "buyer@example.com", litepay.PAID, "2024-01-01 10:00:00",
		models.CartProduct{ProductID: guide.ID, Quantity: 1},
		models.CartProduct{ProductID: keyed.ID, Quantity: 2},
		// A line whose product row is gone: the id is all the cart keeps, so
		// there is nothing left to name it by.
		models.CartProduct{ProductID: "product-deleted", Quantity: 1},
	)
	claimKey(t, ctx, older.ID, claimed.ID)

	addPaidCart(t, db, ctx, "purchase-newer", "buyer@example.com", litepay.PAID, "2024-06-01 10:00:00",
		models.CartProduct{ProductID: external.ID, Quantity: 3})

	// An abandoned cart of the same buyer, a stranger's paid cart: neither is a
	// purchase of this buyer's, and neither may appear.
	addPaidCart(t, db, ctx, "purchase-unpaid", "buyer@example.com", litepay.UNPAID, "2024-07-01 10:00:00",
		models.CartProduct{ProductID: external.ID, Quantity: 1})
	addPaidCart(t, db, ctx, "purchase-stranger", "other@example.com", litepay.PAID, "2024-08-01 10:00:00",
		models.CartProduct{ProductID: external.ID, Quantity: 1})

	// The address is matched the way it was stored on both sides, so a buyer
	// who typed a capital letter at checkout still finds their orders.
	purchases, err := db.CustomerPurchases(ctx, "  BUYER@Example.com ")
	if err != nil {
		t.Fatalf("CustomerPurchases: %v", err)
	}

	if len(purchases) != 2 {
		t.Fatalf("got %d purchases, want 2: %+v", len(purchases), purchases)
	}
	if purchases[0].ID != "purchase-newer" || purchases[1].ID != "purchase-older" {
		t.Errorf("order = [%s %s], want newest first", purchases[0].ID, purchases[1].ID)
	}

	newer := purchases[0]
	if newer.AmountTotal != 7500 || newer.Currency != "EUR" {
		t.Errorf("newer order total = %d %s, want 7500 EUR", newer.AmountTotal, newer.Currency)
	}
	if newer.Created == 0 {
		t.Error("newer order carries no created timestamp")
	}
	if len(newer.Items) != 1 || newer.Items[0].Name != "Paper Map" || newer.Items[0].Quantity != 3 {
		t.Errorf("newer items = %+v", newer.Items)
	}
	if len(newer.Items[0].Files) != 0 || len(newer.Items[0].Codes) != 0 {
		t.Errorf("an api product hands nothing over through the cabinet: %+v", newer.Items[0])
	}

	items := purchases[1].Items
	if len(items) != 2 {
		t.Fatalf("older items = %d, want 2 (the deleted product drops out): %+v", len(items), items)
	}

	var seenGuide, seenKeyed bool
	for _, item := range items {
		switch item.ProductID {
		case guide.ID:
			seenGuide = true
			if item.Slug != "purchase-guide" || item.Digital != "file" || item.Quantity != 1 {
				t.Errorf("guide item = %+v", item)
			}
			if len(item.Files) != 2 {
				t.Fatalf("guide files = %+v, want 2", item.Files)
			}
			// Ordered by the name the buyer will see on disk, and named even
			// when the upload had no name of its own.
			want := []models.CustomerPurchaseFile{
				{ID: unnamed.ID, OrigName: "22222222-2222-4222-8222-222222222222.pdf"},
				{ID: guideFile.ID, OrigName: "castellon.pdf"},
			}
			if !reflect.DeepEqual(item.Files, want) {
				t.Errorf("files = %+v, want %+v", item.Files, want)
			}
		case keyed.ID:
			seenKeyed = true
			if item.Digital != "data" || item.Quantity != 2 {
				t.Errorf("keyed item = %+v", item)
			}
			// Only the key this order claimed. The unclaimed one belongs to
			// nobody yet, and printing it here would hand the buyer the shop's
			// remaining stock.
			if len(item.Codes) != 1 || item.Codes[0] != "KEY-THIS-ONE" {
				t.Errorf("codes = %+v, want just the claimed key", item.Codes)
			}
		default:
			t.Errorf("unexpected item %+v", item)
		}
	}
	if !seenGuide || !seenKeyed {
		t.Errorf("missing items: guide=%v keyed=%v", seenGuide, seenKeyed)
	}

	// An address with no orders gets an empty list, not an error and not
	// somebody else's orders.
	none, err := db.CustomerPurchases(ctx, "nobody@example.com")
	if err != nil {
		t.Fatalf("CustomerPurchases unknown address: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("unknown address returned %+v", none)
	}
}

// TestCustomerOwnsProduct is the entitlement behind every download.
func TestCustomerOwnsProduct(t *testing.T) {
	db, ctx := bootstrap(t)

	product := addProduct(t, db, ctx, "owned-product", "Owned Guide", "file")
	other := addProduct(t, db, ctx, "unbought-product", "Unbought Guide", "file")

	addPaidCart(t, db, ctx, "owns-paid", "buyer@example.com", litepay.PAID, "2024-01-01 10:00:00",
		models.CartProduct{ProductID: product.ID, Quantity: 1})
	addPaidCart(t, db, ctx, "owns-unpaid", "buyer@example.com", litepay.UNPAID, "2024-01-02 10:00:00",
		models.CartProduct{ProductID: other.ID, Quantity: 1})

	tests := []struct {
		name      string
		email     string
		productID string
		want      bool
	}{
		{"paid for it", "buyer@example.com", product.ID, true},
		{"paid for it, addressed differently", " BUYER@example.com", product.ID, true},
		// An order that was never paid is not an entitlement.
		{"left it in an unpaid cart", "buyer@example.com", other.ID, false},
		{"somebody else bought it", "other@example.com", product.ID, false},
		{"does not exist", "buyer@example.com", "no-such-product", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.CustomerOwnsProduct(ctx, tt.email, tt.productID)
			if err != nil {
				t.Fatalf("CustomerOwnsProduct: %v", err)
			}
			if got != tt.want {
				t.Errorf("CustomerOwnsProduct = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEntitledDigitalFile covers what the download endpoint may hand out.
func TestEntitledDigitalFile(t *testing.T) {
	db, ctx := bootstrap(t)

	owned := addProduct(t, db, ctx, "entitled-product", "Owned Guide", "file")
	ownedFile, err := db.AddDigitalFile(ctx, owned.ID, "33333333-3333-4333-8333-333333333333", "pdf", "owned.pdf")
	if err != nil {
		t.Fatalf("add owned file: %v", err)
	}

	unbought := addProduct(t, db, ctx, "unbought-guide", "Unbought Guide", "file")
	unboughtFile, err := db.AddDigitalFile(ctx, unbought.ID, "44444444-4444-4444-8444-444444444444", "pdf", "unbought.pdf")
	if err != nil {
		t.Fatalf("add unbought file: %v", err)
	}

	unpaid := addProduct(t, db, ctx, "unpaid-guide", "In An Unpaid Cart", "file")
	unpaidFile, err := db.AddDigitalFile(ctx, unpaid.ID, "55555555-5555-4555-8555-555555555555", "pdf", "unpaid.pdf")
	if err != nil {
		t.Fatalf("add unpaid file: %v", err)
	}

	addPaidCart(t, db, ctx, "entitled-paid", "buyer@example.com", litepay.PAID, "2024-01-01 10:00:00",
		models.CartProduct{ProductID: owned.ID, Quantity: 1})
	addPaidCart(t, db, ctx, "entitled-unpaid", "buyer@example.com", litepay.UNPAID, "2024-01-02 10:00:00",
		models.CartProduct{ProductID: unpaid.ID, Quantity: 1})

	file, err := db.EntitledDigitalFile(ctx, "buyer@example.com", ownedFile.ID)
	if err != nil {
		t.Fatalf("EntitledDigitalFile own file: %v", err)
	}
	if file.OrigName != "owned.pdf" || file.Name == "" || file.Ext != "pdf" {
		t.Errorf("file = %+v", file)
	}

	// Every way of not being entitled has to answer the same thing, or the
	// endpoint tells a stranger which file ids exist.
	for _, tt := range []struct {
		name   string
		email  string
		fileID string
	}{
		{"not bought", "other@example.com", ownedFile.ID},
		{"never paid for that product", "buyer@example.com", unpaidFile.ID},
		{"file of a product nobody in this account bought", "buyer@example.com", unboughtFile.ID},
		{"no such file", "buyer@example.com", "no-such-file-id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.EntitledDigitalFile(ctx, tt.email, tt.fileID); !errors.Is(err, errors.ErrProductNotFound) {
				t.Errorf("got %v, want ErrProductNotFound", err)
			}
		})
	}
}
