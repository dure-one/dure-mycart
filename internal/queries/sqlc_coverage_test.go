//go:build sqlc

package queries

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ported maps each plain file the sqlc build leaves out to the git blob hash
// (`git hash-object <file>`) it had when its <name>_sqlc.go twin was last
// brought in line with it.
//
// After merging upstream, a mismatch here means the plain file changed: port
// the change into the twin, then update the hash.
var ported = map[string]string{
	"auth.go":    "c60a32c917cd7caf4680304af0f9c1005ec3272a",
	"session.go": "7de97a1e11771895b80ecff5802df637a2d4e2ff",
	"install.go": "afb71a908e6c7ebea57a364ebd50e850c1475890",
	"seo.go":     "0a1a20fe444d613fd17a29c190c1e5d4bfc18a9f",
	"pages.go":   "55dfe91a96c1171b6c91591e5b5d00f6cd408c6a",
	"setting.go": "e06923a703aba61851ed1abf15f240e072fd2256",

	"customer.go":               "cf55dcc154c2143991da89286ad4f4d13bf74aae",
	"customer_account.go":       "b5b73ea228187092fd096bccfdadd6519db0d780",
	"customer_list.go":          "bb3a5f2cba218ba812eaca58e47cecf63114c0d3",
	"customer_purchase_read.go": "8e873a4052feb3aa838b4e47fbeffe89a0adc4ea",

	"cart.go":        "a40afc930c8f0c06b7e4165623b43da977d26438",
	"cart_list.go":   "4a735b447f0a77884cf537eae9d63dcdb8b571a4",
	"cart_letter.go": "b944c7c0823f8265a0502a7a28d2b776ea34edba",

	"products.go":                "e6fd6b759aa534e82a7c609012309e678ec0325e",
	"products_digital.go":        "85d0f20d04eb7c8822077c597c7934ee601b4d01",
	"products_get.go":            "2da20186b9128f4fb34870443f63983b5a55c3ec",
	"products_get_variants.go":   "b008b86a60fffa595474c2503ddac8b747a5a6bc",
	"products_images.go":         "79a3ab2b1c56a9a5c5f54f5da2529ab26d33de21",
	"products_list.go":           "68af101af3a846cbc45dcb0ec77c0c9447d5e397",
	"products_variants_read.go":  "ff33eb4f0d54ed2a030bb5e8c5a96227fa2b88e0",
	"products_variants_write.go": "9ae068ee0cbdd387832e6ae7887cdaa3a58a0ad5",
	"products_write.go":          "68e474dcbf2765c4ac8424695f27cfb36e6a07ba",
	"product_image.go":           "a230ae02935eee307f672a24fe3cf54ebff9fbdf",
}

// mirrored maps files outside this package whose logic a sqlc twin re-implements
// over generated queries, to the git blob hash they had when it was copied.
// They carry no build tag: the plain build still uses them.
var mirrored = map[string]string{
	// GenerateUniqueSlug in products_sqlc.go.
	"../../pkg/slugify/slug.go": "27633ea9e2e56ef9b0630c791955517c92b87ff0",
}

// gitBlobHash is what `git hash-object` prints for content.
func gitBlobHash(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// TestSQLCPortedFilesUnchanged fails when an upstream merge changes a plain
// file whose logic the sqlc build carries a copy of.
func TestSQLCPortedFilesUnchanged(t *testing.T) {
	for name, want := range ported {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("%s: %v (renamed or split upstream? re-port it)", name, err)
			continue
		}
		if !strings.HasPrefix(string(src), "//go:build !sqlc") {
			t.Errorf("%s lost its //go:build !sqlc line", name)
		}
		if got := gitBlobHash(src); got != want {
			t.Errorf("%s changed (hash %s, ported at %s): port the change into %s_sqlc.go, then update the hash",
				name, got, want, strings.TrimSuffix(name, ".go"))
		}
	}

	for name, want := range mirrored {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("%s: %v (renamed upstream? re-port it)", name, err)
			continue
		}
		if got := gitBlobHash(src); got != want {
			t.Errorf("%s changed (hash %s, mirrored at %s): port the change into its sqlc twin, then update the hash",
				name, got, want)
		}
	}
}

// unported lists the plain files that the sqlc build still compiles and runs.
// Every query group is ported, so it is empty; a file merged from upstream may
// be parked here while its <name>_sqlc.go twin is written.
//
// Port a file: write its queries under sqlcgen/queries/, add the sqlc file,
// put `//go:build !sqlc` on the plain file, record it in ported and delete its
// name here.
var unported = map[string]bool{
	"responder_archive.go":   true,
	"responder_contacts.go":  true,
	"responder_crontab.go":   true,
	"responder_messages.go":  true,
	"responder_workflows.go": true,
}

var runsSQL = regexp.MustCompile(`\.(QueryContext|ExecContext|QueryRowContext|PrepareContext|BeginTx)\(`)

// TestSQLCCoverage fails when a file without a build tag runs SQL and is not
// listed in unported. That is how a file merged from upstream shows up: it
// arrives untagged, so the sqlc build would otherwise compile it and quietly
// run its plain SQL.
func TestSQLCCoverage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	var untracked []string
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(src), "//go:build") || !runsSQL.Match(src) {
			continue
		}
		seen[name] = true
		if !unported[name] {
			untracked = append(untracked, name)
		}
	}

	if len(untracked) > 0 {
		t.Errorf("untagged files run plain SQL in the sqlc build: %v\n"+
			"port them (or, for now, add them to unported)", untracked)
	}

	var stale []string
	for name := range unported {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("listed in unported but no longer untagged plain SQL: %v — remove them from the list", stale)
	}
}
