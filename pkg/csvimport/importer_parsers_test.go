package csvimport

import (
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/models"
)

// The variant string is a small grammar of its own:
//
//	[Size:S;M][Color:Black]=>S,Black,500,10,SKU-1|M,Black,700,5,SKU-2
//
// Each helper below handles one production of it, so each is tested on its own
// as well as through parseVariantsB3.

func TestExtractVariantString(t *testing.T) {
	imp := NewCSVImporter(nil)

	tests := []struct {
		name      string
		record    []string
		headerMap map[string]int
		want      string
		wantOK    bool
	}{
		{"no variants column", []string{"x"}, map[string]int{"name": 0}, "", false},
		{"column missing from the record", []string{"x"}, map[string]int{"variants": 3}, "", false},
		{"empty value", []string{""}, map[string]int{"variants": 0}, "", false},
		{"only whitespace", []string{"   "}, map[string]int{"variants": 0}, "", false},
		{"trimmed", []string{"  [Size:S]=>S,0,1,sku  "}, map[string]int{"variants": 0}, "[Size:S]=>S,0,1,sku", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := imp.extractVariantString(tt.record, tt.headerMap)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("extractVariantString = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestSplitVariantSections(t *testing.T) {
	imp := NewCSVImporter(nil)

	t.Run("splits on the arrow", func(t *testing.T) {
		options, variants, err := imp.splitVariantSections("[Size:S]=>S,0,1,sku")
		if err != nil {
			t.Fatalf("split: %v", err)
		}
		if options != "[Size:S]" || variants != "S,0,1,sku" {
			t.Errorf("got (%q, %q)", options, variants)
		}
	})

	t.Run("missing arrow is an error", func(t *testing.T) {
		_, _, err := imp.splitVariantSections("[Size:S]")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "missing '=>' separator") {
			t.Errorf("error = %q", err)
		}
	})
}

func TestParseOptionDefinition(t *testing.T) {
	imp := NewCSVImporter(nil)

	t.Run("parses a name and its values", func(t *testing.T) {
		got, err := imp.parseOptionDefinition(" Size : S ; M ; L ", 1)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got.Name != "Size" || got.Position != 1 {
			t.Errorf("got %+v", got)
		}
		if len(got.Values) != 3 {
			t.Fatalf("values = %+v, want 3", got.Values)
		}
		if got.Values[0].Value != "S" || got.Values[2].Value != "L" {
			t.Errorf("values = %+v", got.Values)
		}
		// The position is the value's index in the definition, which is what
		// the admin UI orders them by.
		if got.Values[0].Position != 0 || got.Values[2].Position != 2 {
			t.Errorf("positions = %+v", got.Values)
		}
	})

	t.Run("empty values between separators are dropped", func(t *testing.T) {
		got, err := imp.parseOptionDefinition("Size:S;;M;", 0)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got.Values) != 2 {
			t.Errorf("values = %+v, want 2", got.Values)
		}
	})

	t.Run("missing colon is an error", func(t *testing.T) {
		_, err := imp.parseOptionDefinition("Size", 0)
		if err == nil || !strings.Contains(err.Error(), "missing ':' separator") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("no values is an error", func(t *testing.T) {
		_, err := imp.parseOptionDefinition("Size:", 0)
		if err == nil || !strings.Contains(err.Error(), "at least one value") {
			t.Errorf("error = %v", err)
		}
	})
}

func TestParseOptionDefinitions(t *testing.T) {
	imp := NewCSVImporter(nil)

	t.Run("one option", func(t *testing.T) {
		got, err := imp.parseOptionDefinitions("[Size:S;M]")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got) != 1 || got[0].Name != "Size" || got[0].Position != 0 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("three options are the maximum", func(t *testing.T) {
		got, err := imp.parseOptionDefinitions("[A:1][B:2][C:3]")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d options, want 3", len(got))
		}
		for i, option := range got {
			if option.Position != i {
				t.Errorf("option %d has position %d", i, option.Position)
			}
		}
	})

	t.Run("a fourth option is rejected", func(t *testing.T) {
		_, err := imp.parseOptionDefinitions("[A:1][B:2][C:3][D:4]")
		if err == nil || !strings.Contains(err.Error(), "maximum 3 options") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("unmatched brackets are rejected", func(t *testing.T) {
		_, err := imp.parseOptionDefinitions("[Size:S")
		if err == nil || !strings.Contains(err.Error(), "unmatched brackets") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("no options at all is rejected", func(t *testing.T) {
		_, err := imp.parseOptionDefinitions("no brackets here")
		if err == nil || !strings.Contains(err.Error(), "no valid options") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("text before the first bracket is ignored", func(t *testing.T) {
		got, err := imp.parseOptionDefinitions("junk [Size:S] trailing")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got) != 1 || got[0].Name != "Size" {
			t.Errorf("got %+v", got)
		}
	})
}

func TestParseVariantData(t *testing.T) {
	imp := NewCSVImporter(nil)
	options := []models.ProductOption{{Name: "Size"}, {Name: "Color"}}

	t.Run("parses one row", func(t *testing.T) {
		got, err := imp.parseVariantData(" S , Black , 500 , 10 , SKU-1 ", options)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got.OptionValues["Size"] != "S" || got.OptionValues["Color"] != "Black" {
			t.Errorf("option values = %+v", got.OptionValues)
		}
		if got.PriceSurcharge != 500 || got.Quantity != 10 || got.SKU != "SKU-1" {
			t.Errorf("got %+v", got)
		}
		// A variant parsed from a CSV is live: the import is what creates it.
		if !got.Active {
			t.Error("the variant is inactive")
		}
	})

	tests := []struct {
		name, row, want string
	}{
		{"too few parts", "S,Black,500", "expected 5 parts, got 3"},
		{"too many parts", "S,Black,500,10,SKU-1,extra", "expected 5 parts, got 6"},
		{"bad price", "S,Black,free,10,SKU-1", "invalid price surcharge: free"},
		{"bad quantity", "S,Black,500,many,SKU-1", "invalid quantity: many"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := imp.parseVariantData(tt.row, options)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestParseVariantDataRows(t *testing.T) {
	imp := NewCSVImporter(nil)
	options := []models.ProductOption{{Name: "Size"}}

	t.Run("several rows", func(t *testing.T) {
		got, err := imp.parseVariantDataRows("S,100,1,sku-s|M,200,2,sku-m", options)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got) != 2 || got[0].SKU != "sku-s" || got[1].SKU != "sku-m" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("blank rows are skipped", func(t *testing.T) {
		got, err := imp.parseVariantDataRows("  |S,100,1,sku|  ", options)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d variants, want 1", len(got))
		}
	})

	t.Run("an empty section yields no variants", func(t *testing.T) {
		got, err := imp.parseVariantDataRows("", options)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d variants, want 0", len(got))
		}
	})

	t.Run("a bad row fails the whole section", func(t *testing.T) {
		if _, err := imp.parseVariantDataRows("S,100,1,sku|broken", options); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestParseVariantsB3(t *testing.T) {
	imp := NewCSVImporter(nil)

	t.Run("no variant column means no variants", func(t *testing.T) {
		hasVariants, options, variants, err := imp.parseVariantsB3([]string{"x"}, map[string]int{"name": 0})
		if err != nil || hasVariants || options != nil || variants != nil {
			t.Errorf("got (%v, %v, %v, %v)", hasVariants, options, variants, err)
		}
	})

	t.Run("a complete row parses end to end", func(t *testing.T) {
		record := []string{"[Size:S;M][Color:Black]=>S,Black,500,10,SKU-1|M,Black,700,5,SKU-2"}
		headerMap := map[string]int{"variants": 0}

		hasVariants, options, variants, err := imp.parseVariantsB3(record, headerMap)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if !hasVariants {
			t.Fatal("hasVariants is false")
		}
		if len(options) != 2 || options[0].Name != "Size" || options[1].Name != "Color" {
			t.Fatalf("options = %+v", options)
		}
		if len(variants) != 2 {
			t.Fatalf("variants = %+v", variants)
		}
		if variants[0].SKU != "SKU-1" || variants[1].PriceSurcharge != 700 {
			t.Errorf("variants = %+v", variants)
		}
	})

	t.Run("errors are reported rather than swallowed", func(t *testing.T) {
		_, _, _, err := imp.parseVariantsB3([]string{"[Size:S]"}, map[string]int{"variants": 0})
		if err == nil || !strings.Contains(err.Error(), "missing '=>' separator") {
			t.Errorf("error = %v", err)
		}
	})
}

func TestParseImages(t *testing.T) {
	imp := NewCSVImporter(nil)

	t.Run("nothing to do", func(t *testing.T) {
		product := &models.Product{}
		imp.parseImages(product, []string{"x"}, map[string]int{"name": 0})
		imp.parseImages(product, []string{""}, map[string]int{"images": 0})
		imp.parseImages(product, []string{"x"}, map[string]int{"images": 9})
		if len(product.Images) != 0 {
			t.Errorf("images = %+v", product.Images)
		}
	})

	t.Run("pipe separated urls", func(t *testing.T) {
		product := &models.Product{}
		imp.parseImages(product, []string{"a.jpg | b.png| |c"}, map[string]int{"images": 0})

		if len(product.Images) != 3 {
			t.Fatalf("images = %+v, want 3", product.Images)
		}
		if product.Images[0].OrigName != "a.jpg" || product.Images[0].Ext != "jpg" {
			t.Errorf("first image = %+v", product.Images[0])
		}
		if product.Images[1].Ext != "png" {
			t.Errorf("second image = %+v", product.Images[1])
		}
		// A name with no dot has no extension rather than a panic.
		if product.Images[2].Ext != "" {
			t.Errorf("third image = %+v", product.Images[2])
		}
		for _, image := range product.Images {
			if image.ID == "" || image.Name == "" {
				t.Errorf("image is missing a generated id or name: %+v", image)
			}
		}
		if product.Images[0].ID == product.Images[1].ID {
			t.Error("two images share an id")
		}
	})
}

func TestParseAttributes(t *testing.T) {
	imp := NewCSVImporter(nil)

	t.Run("nothing to do", func(t *testing.T) {
		product := &models.Product{}
		imp.parseAttributes(product, []string{"x"}, map[string]int{"name": 0})
		imp.parseAttributes(product, []string{""}, map[string]int{"attributes": 0})
		if len(product.Attributes) != 0 {
			t.Errorf("attributes = %+v", product.Attributes)
		}
	})

	t.Run("pipe separated values are trimmed and blanks dropped", func(t *testing.T) {
		product := &models.Product{}
		imp.parseAttributes(product, []string{" red | blue | | green "}, map[string]int{"attributes": 0})

		want := []string{"red", "blue", "green"}
		if len(product.Attributes) != len(want) {
			t.Fatalf("attributes = %+v, want %v", product.Attributes, want)
		}
		for i, attribute := range product.Attributes {
			if attribute != want[i] {
				t.Errorf("attributes = %+v, want %v", product.Attributes, want)
			}
		}
	})
}

// Every generated id has to be filled in and wired to its parent: the import
// inserts these rows as they are.
func TestGenerateProductIDs(t *testing.T) {
	imp := NewCSVImporter(nil)
	product := &models.Product{
		Options: []models.ProductOption{
			{Values: []models.ProductOptionValue{{}, {}}},
			{Values: []models.ProductOptionValue{{}}},
		},
		Variants: []models.ProductVariant{{}, {}},
	}

	imp.generateProductIDs(product)

	if product.ID == "" {
		t.Fatal("the product has no id")
	}

	seen := map[string]bool{product.ID: true}
	check := func(what, id string) {
		t.Helper()
		if id == "" {
			t.Errorf("%s has no id", what)
			return
		}
		if seen[id] {
			t.Errorf("%s reuses the id %q", what, id)
		}
		seen[id] = true
	}

	for i, option := range product.Options {
		check("option", option.ID)
		if option.ProductID != product.ID {
			t.Errorf("option %d points at product %q, want %q", i, option.ProductID, product.ID)
		}
		for j, value := range option.Values {
			check("option value", value.ID)
			if value.OptionID != option.ID {
				t.Errorf("value %d of option %d points at %q, want %q", j, i, value.OptionID, option.ID)
			}
		}
	}

	for i, variant := range product.Variants {
		check("variant", variant.ID)
		if variant.ProductID != product.ID {
			t.Errorf("variant %d points at product %q, want %q", i, variant.ProductID, product.ID)
		}
	}
}
