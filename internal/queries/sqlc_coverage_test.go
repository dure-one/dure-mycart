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
	"auth.go":    "3b0ae770fddc89612a1224a4e42f829b04f3df5f",
	"session.go": "7de97a1e11771895b80ecff5802df637a2d4e2ff",
	"install.go": "0d67659ac8b91ecc2a2fefb946980f1665673270",
	"pages.go":   "f8f1dda6a92c3fc899be28d3ea1e464c6eb609fe",
	"setting.go": "0886e9ff9cf6f6ac95b51392e880334c714a0bac",

	"customer.go":               "374d2d3632f235463b6374c08a95bff6c1b91cb2",
	"customer_account.go":       "456f89539e790efa14e51f02757fbfe14bf216c4",
	"customer_list.go":          "5fe3088faf49af242fb6c6629b28dc5b717f9213",
	"customer_purchase_read.go": "0b0771747e503b4b2043ccced5c555fa05c77a1f",

	"cart.go":        "bccddcf69a254de4d1a00fedb95c001e66639e57",
	"cart_list.go":   "f2787194f1ebf79c672f884861c1e1a234d3f134",
	"cart_letter.go": "b95174820f84634b39ddc08f4db7141509b62ca1",

	"products.go":                "f1e4eb6555da2dadb5acb529d5f33e7fbc6e889f",
	"products_digital.go":        "d0313bef18685d9558f1d332adb8e28e06ce3555",
	"products_get.go":            "8e2643b71070bdb4e6a08594dc98cf11db79f900",
	"products_get_variants.go":   "3e257fd1f95b5edf6348426f661e241be53f1ecb",
	"products_images.go":         "4547f2db8a263075dce5541ae0228a8539eed501",
	"products_list.go":           "64e400b4472db9b387f7c62c7ff6036e516cbc53",
	"products_variants_read.go":  "28ab0752b6aa385eef53985d00cd22b1f00c45ae",
	"products_variants_write.go": "cc98081eeb44f22ceceb12bcb3d619e3a49f0b97",
	"products_write.go":          "3d77a0a4229442771f9d6452a4e11ee1404b4d78",
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
var unported = map[string]bool{}

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
