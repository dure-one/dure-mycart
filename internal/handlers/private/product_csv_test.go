package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/csvimport"
)

func TestEscapeCSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text is untouched", "Summit Seekers", "Summit Seekers"},
		{"empty stays empty", "", ""},
		{"comma", "a,b", `"a,b"`},
		{"quote", `say "hi"`, `"say ""hi"""`},
		{"newline", "line1\nline2", "\"line1\nline2\""},
		{"carriage return", "line1\r\nline2", "\"line1\r\nline2\""},
		{"only a quote", `"`, `""""`},
		{"semicolon needs no quoting", "a;b", "a;b"},
		{"pipe needs no quoting", "a|b", "a|b"},
		{"leading space is kept as is", " padded", " padded"},
		{"unicode", "Гора, Сияние", `"Гора, Сияние"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := escapeCSV(tt.in); got != tt.want {
				t.Errorf("escapeCSV(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildImageString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		images []models.File
		want   string
	}{
		{"no images", nil, ""},
		{"empty slice", []models.File{}, ""},
		{
			name:   "original name wins",
			images: []models.File{{Name: "uuid", Ext: "png", OrigName: "photo.png"}},
			want:   "photo.png",
		},
		{
			name:   "name and extension when there is no original name",
			images: []models.File{{Name: "uuid", Ext: "jpg"}},
			want:   "uuid.jpg",
		},
		{
			name:   "name alone when there is no extension",
			images: []models.File{{Name: "uuid"}},
			want:   "uuid",
		},
		{
			name: "joined with a pipe",
			images: []models.File{
				{Name: "a", Ext: "png", OrigName: "one.png"},
				{Name: "b", Ext: "png", OrigName: "two.png"},
			},
			want: "one.png|two.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildImageString(tt.images); got != tt.want {
				t.Errorf("buildImageString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildAttributeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attributes []string
		want       string
	}{
		{"none", nil, ""},
		{"empty slice", []string{}, ""},
		{"one", []string{"cotton"}, "cotton"},
		{"several joined with a pipe", []string{"cotton", "blue", "large"}, "cotton|blue|large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildAttributeString(tt.attributes); got != tt.want {
				t.Errorf("buildAttributeString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildVariantsB3String(t *testing.T) {
	t.Parallel()

	option := func(name string, values ...string) models.ProductOption {
		opts := make([]models.ProductOptionValue, len(values))
		for i, v := range values {
			opts[i] = models.ProductOptionValue{Value: v}
		}
		return models.ProductOption{Name: name, Values: opts}
	}

	tests := []struct {
		name    string
		product models.Product
		want    string
	}{
		{
			name:    "no variants flag",
			product: models.Product{Name: "plain"},
			want:    "",
		},
		{
			name:    "variants flag without options",
			product: models.Product{HasVariants: true},
			want:    "",
		},
		{
			name: "options without values are skipped",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{{Name: "Size"}},
			},
			want: "",
		},
		{
			name: "options but no variant rows",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{option("Size", "S", "M")},
			},
			want: "[Size:S;M]=>",
		},
		{
			name: "two options and one variant",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{option("Size", "S", "M"), option("Color", "Red")},
				Variants: []models.ProductVariant{{
					SKU:            "SKU-1",
					OptionValues:   map[string]string{"Size": "M", "Color": "Red"},
					PriceSurcharge: 500,
					Quantity:       7,
				}},
			},
			want: "[Size:S;M][Color:Red]=>M,Red,500,7,SKU-1",
		},
		{
			name: "variant order follows the option order, not the map",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{option("Color", "Red"), option("Size", "S")},
				Variants: []models.ProductVariant{{
					SKU:          "SKU-2",
					OptionValues: map[string]string{"Size": "S", "Color": "Red"},
					Quantity:     1,
				}},
			},
			want: "[Color:Red][Size:S]=>Red,S,0,1,SKU-2",
		},
		{
			name: "a variant missing one option value emits fewer fields",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{option("Size", "S"), option("Color", "Red")},
				Variants: []models.ProductVariant{{
					SKU:          "SKU-3",
					OptionValues: map[string]string{"Size": "S"},
					Quantity:     2,
				}},
			},
			want: "[Size:S][Color:Red]=>S,0,2,SKU-3",
		},
		{
			name: "variants are joined with a pipe",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{option("Size", "S", "M")},
				Variants: []models.ProductVariant{
					{SKU: "S", OptionValues: map[string]string{"Size": "S"}, Quantity: 1},
					{SKU: "M", OptionValues: map[string]string{"Size": "M"}, Quantity: 2},
				},
			},
			want: "[Size:S;M]=>S,0,1,S|M,0,2,M",
		},
		{
			name: "a variant with no SKU still emits all five fields",
			product: models.Product{
				HasVariants: true,
				Options:     []models.ProductOption{option("Size", "S")},
				Variants:    []models.ProductVariant{{OptionValues: map[string]string{"Size": "S"}, Quantity: 3}},
			},
			want: "[Size:S]=>S,0,3,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildVariantsB3String(tt.product); got != tt.want {
				t.Errorf("buildVariantsB3String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// csvUpload builds a multipart body carrying a CSV file, the shape both import
// endpoints read.
func csvUpload(t *testing.T, contents string) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="products.csv"`)
	hdr.Set("Content-Type", "text/csv")

	part, err := w.CreatePart(hdr)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := io.WriteString(part, contents); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, w.FormDataContentType()
}

// postCSV sends a CSV upload to path.
func postCSV(t *testing.T, app *fiber.App, path, contents string) *http.Response {
	t.Helper()

	body, contentType := csvUpload(t, contents)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// postEnvelope sends a CSV upload and decodes the webutil envelope around the
// answer. Result is kept raw: the failure path carries a string there while the
// success path carries an import result.
func postEnvelope(t *testing.T, app *fiber.App, path, contents string) (int, csvEnvelope, []byte) {
	t.Helper()

	resp := postCSV(t, app, path, contents)
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var env csvEnvelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env, raw
}

type csvEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// importResult decodes the success payload.
func (e csvEnvelope) importResult(t *testing.T) csvimport.ImportResult {
	t.Helper()

	var out csvimport.ImportResult
	if err := json.Unmarshal(e.Result, &out); err != nil {
		t.Fatalf("result is not an import result: %s (%v)", e.Result, err)
	}
	return out
}

// reason returns the message a rejected upload carries.
func (e csvEnvelope) reason() string {
	var s string
	if err := json.Unmarshal(e.Result, &s); err == nil {
		return s
	}
	return e.Message
}

// productBySlug loads a product the way the importer keys them: the admin
// endpoints address products by id, but a CSV row is identified by its slug.
func productBySlug(t *testing.T, slug string) *models.Product {
	t.Helper()

	var id string
	err := queries.DB().ProductQueries.DB.QueryRowContext(context.Background(),
		`SELECT id FROM product WHERE slug = ?`, slug).Scan(&id)
	if err != nil {
		t.Fatalf("no product with slug %q: %v", slug, err)
	}

	product, err := queries.DB().Product(context.Background(), true, id)
	if err != nil {
		t.Fatalf("load product %s: %v", id, err)
	}
	return product
}

func TestImportPreview(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/api/_/products/import/preview", ImportPreview)

	t.Run("a missing file is a bad request", func(t *testing.T) {
		body, contentType := csvUpload(t, "")
		_ = body

		req := httptest.NewRequest(http.MethodPost, "/api/_/products/import/preview", strings.NewReader(""))
		req.Header.Set("Content-Type", contentType)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		testutil.AssertStatusCode(t, resp.StatusCode, http.StatusBadRequest)
	})

	t.Run("a new product is reported as an addition", func(t *testing.T) {
		csv := "name,slug,amount,digital\n" +
			"Brand New Product,brand-new-product,1999,file\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import/preview", csv)
		testutil.AssertStatusCode(t, status, http.StatusOK)

		got := env.importResult(t)
		if got.TotalRows != 1 || got.ToAdd != 1 || got.ToUpdate != 0 {
			t.Errorf("result = %+v, want one row to add", got)
		}
		if got.Skipped != 0 || len(got.Errors) != 0 {
			t.Errorf("result = %+v, want no errors", got)
		}
	})

	t.Run("a known slug is reported as an update", func(t *testing.T) {
		// The fixtures ship a product under this slug.
		csv := "name,slug,amount,digital\n" +
			"Existing Product,url1,1999,file\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import/preview", csv)
		testutil.AssertStatusCode(t, status, http.StatusOK)

		if got := env.importResult(t); got.ToUpdate != 1 {
			t.Errorf("result = %+v, want one row to update", got)
		}
	})

	t.Run("a missing required column is rejected", func(t *testing.T) {
		csv := "name,slug,amount\nNo Digital Column,nope,100\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import/preview", csv)
		testutil.AssertStatusCode(t, status, http.StatusBadRequest)
		if !strings.Contains(env.reason(), "missing required column: digital") {
			t.Errorf("response = %+v, want it to name the missing column", env)
		}
	})

	t.Run("a row without a name is skipped and reported", func(t *testing.T) {
		csv := "name,slug,amount,digital\n" +
			",no-name,100,file\n" +
			"Good Row,good-row,100,file\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import/preview", csv)
		testutil.AssertStatusCode(t, status, http.StatusOK)

		got := env.importResult(t)
		if got.TotalRows != 2 {
			t.Errorf("total rows = %d, want 2", got.TotalRows)
		}
		if got.Skipped != 1 || got.ToAdd != 1 {
			t.Errorf("result = %+v, want one skipped and one added", got)
		}
		if len(got.Errors) != 1 || got.Errors[0].Line != 2 {
			t.Errorf("errors = %+v, want one error on line 2", got.Errors)
		}
	})

	t.Run("an unparsable row is reported without failing the request", func(t *testing.T) {
		// An unterminated quote makes encoding/csv reject the row.
		csv := "name,slug,amount,digital\n" +
			`"unterminated,broken-row,100,file` + "\n" +
			"Good Row,good-row,100,file\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import/preview", csv)
		testutil.AssertStatusCode(t, status, http.StatusOK)

		got := env.importResult(t)
		if len(got.Errors) == 0 {
			t.Fatalf("result = %+v, want the broken row to be reported", got)
		}
		if !strings.Contains(got.Errors[0].Message, "CSV parse error") {
			t.Errorf("error = %q, want it to name the parse failure", got.Errors[0].Message)
		}
	})
}

func TestImportProducts(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/api/_/products/import", ImportProducts)

	t.Run("a missing file is a bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/_/products/import", strings.NewReader(""))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=nothing")
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		testutil.AssertStatusCode(t, resp.StatusCode, http.StatusBadRequest)
	})

	t.Run("a missing required column is rejected before anything is written", func(t *testing.T) {
		csv := "name,slug,amount\nNo Digital Column,nope,100\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import", csv)
		testutil.AssertStatusCode(t, status, http.StatusBadRequest)
		if !strings.Contains(env.reason(), "missing required column") {
			t.Errorf("response = %+v, want it to name the missing column", env)
		}

		if queries.DB().IsProduct(context.Background(), "nope") {
			t.Error("the rejected row must not have been imported")
		}
	})

	t.Run("a new product is written", func(t *testing.T) {
		csv := "name,slug,amount,digital\n" +
			"Importable Product,importable-product,1234,file\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import", csv)
		testutil.AssertStatusCode(t, status, http.StatusOK)

		got := env.importResult(t)
		if got.Imported != 1 || got.Skipped != 0 {
			t.Fatalf("result = %+v, want one product imported", got)
		}

		product := productBySlug(t, "importable-product")
		if product.Name != "Importable Product" || product.Amount != 1234 {
			t.Errorf("product = %q %d, want the imported values", product.Name, product.Amount)
		}
	})

	// Re-importing a row that already exists is counted as skipped and changes
	// nothing: csvimport.Import has no update path yet (its own comment says the
	// update logic "would go here"). The preview counts such rows as to_update,
	// so an operator reading "1 product will be updated" gets a skipped row and
	// an unchanged product. This pins the behaviour so the day the update path
	// lands, this test is what says so.
	t.Run("a repeated import skips the row instead of updating it", func(t *testing.T) {
		csv := "name,slug,amount,digital\n" +
			"Importable Product Renamed,importable-product,4321,file\n"

		status, env, _ := postEnvelope(t, app, "/api/_/products/import", csv)
		testutil.AssertStatusCode(t, status, http.StatusOK)

		got := env.importResult(t)
		if got.Skipped != 1 || got.Updated != 0 || got.Imported != 0 {
			t.Fatalf("result = %+v, want the existing row skipped", got)
		}

		product := productBySlug(t, "importable-product")
		if product.Name != "Importable Product" || product.Amount != 1234 {
			t.Fatalf("product = %q %d; if the update path was just implemented, update this test",
				product.Name, product.Amount)
		}
	})
}

func TestExportProductsHeaders(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/api/_/products/export", ExportProducts)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/products/export", "", "")
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read export: %v", err)
	}

	testutil.AssertStatusCode(t, resp.StatusCode, http.StatusOK)

	if got := resp.Header.Get("Content-Type"); got != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", got)
	}
	if got := resp.Header.Get("Content-Disposition"); got != "attachment; filename=products.csv" {
		t.Errorf("Content-Disposition = %q", got)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	wantHeader := "name,slug,brief,description,images,attributes,amount,quantity,sku,variants,digital,active"
	if lines[0] != wantHeader {
		t.Fatalf("header = %q, want %q", lines[0], wantHeader)
	}

	// The fixtures hold eight products.
	if got := len(lines) - 1; got != 8 {
		t.Errorf("exported %d rows, want 8", got)
	}
}

// The export and the import are two halves of one contract: whatever the export
// writes, the importer has to be able to read back. This walks a product with
// variants, images and attributes through the pair.
func TestExportImportRoundTrip(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/api/_/products/export", ExportProducts)
	app.Post("/api/_/products", AddProduct)

	// Commas, quotes and pipes in the text are what the escaping exists for.
	payload := `{
		"name": "Round Trip, the \"Third\"",
		"slug": "round-trip",
		"brief": "Short, with a comma",
		"description": "Line one\nLine two, with \"quotes\"",
		"amount": 3300,
		"quantity": 4,
		"sku": "SKU,ROOT",
		"active": true,
		"has_variants": true,
		"digital": {"type": "file"},
		"attributes": ["cotton", "blue"],
		"options": [
			{"name": "Size", "values": [{"value": "Small"}, {"value": "Medium"}]},
			{"name": "Color", "values": [{"value": "Red"}]}
		],
		"variants": [
			{"sku": "RT-S-R", "option_values": {"Size": "Small", "Color": "Red"}, "price_surcharge": 0, "quantity": 10},
			{"sku": "RT-M-R", "option_values": {"Size": "Medium", "Color": "Red"}, "price_surcharge": 250, "quantity": 5}
		]
	}`
	testutil.AssertStatus(t, testutil.DoRequest(t, app, http.MethodPost, "/api/_/products", payload, ""), http.StatusOK)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/products/export", "", "")
	exported, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	testutil.AssertStatusCode(t, resp.StatusCode, http.StatusOK)

	// Parse the export with the importer the import endpoint uses.
	importer := csvimport.NewCSVImporter(queries.DB().ProductQueries.DB)
	result, products, err := importer.ValidateAndPreview(bytes.NewReader(exported))
	if err != nil {
		t.Fatalf("the exported CSV cannot be parsed back: %v", err)
	}
	if len(result.Errors) != 0 || result.Skipped != 0 {
		t.Fatalf("round trip produced errors: %+v", result)
	}

	var found *models.Product
	for i := range products {
		if products[i].Slug == "round-trip" {
			found = &products[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("the exported CSV does not contain the product; rows: %d", result.TotalRows)
	}

	if found.Name != `Round Trip, the "Third"` {
		t.Errorf("name = %q, want the original with commas and quotes", found.Name)
	}
	if found.Brief != "Short, with a comma" {
		t.Errorf("brief = %q, want the original", found.Brief)
	}
	if found.Amount != 3300 {
		t.Errorf("amount = %d, want 3300", found.Amount)
	}
	if got := strings.Join(found.Attributes, "|"); got != "cotton|blue" {
		t.Errorf("attributes = %q, want cotton|blue", got)
	}

	if !found.HasVariants {
		t.Fatal("the product came back without variants")
	}
	if len(found.Options) != 2 {
		t.Fatalf("options = %+v, want two", found.Options)
	}
	if found.Options[0].Name != "Size" || len(found.Options[0].Values) != 2 {
		t.Errorf("first option = %+v, want Size with two values", found.Options[0])
	}
	if found.Options[1].Name != "Color" {
		t.Errorf("second option = %+v, want Color", found.Options[1])
	}
	if len(found.Variants) != 2 {
		t.Fatalf("variants = %+v, want two", found.Variants)
	}

	bySKU := map[string]models.ProductVariant{}
	for _, v := range found.Variants {
		bySKU[v.SKU] = v
	}
	small, ok := bySKU["RT-S-R"]
	if !ok {
		t.Fatalf("variant RT-S-R is missing from %+v", bySKU)
	}
	if small.PriceSurcharge != 0 || small.Quantity != 10 {
		t.Errorf("variant RT-S-R = %+v, want no surcharge and 10 in stock", small)
	}
	if small.OptionValues["Size"] != "Small" || small.OptionValues["Color"] != "Red" {
		t.Errorf("variant RT-S-R option values = %+v", small.OptionValues)
	}
	medium, ok := bySKU["RT-M-R"]
	if !ok {
		t.Fatalf("variant RT-M-R is missing from %+v", bySKU)
	}
	if medium.PriceSurcharge != 250 || medium.Quantity != 5 {
		t.Errorf("variant RT-M-R = %+v, want a 250 surcharge and 5 in stock", medium)
	}
}
