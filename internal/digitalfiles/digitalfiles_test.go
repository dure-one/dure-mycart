package digitalfiles

import (
	"mime"
	"path/filepath"
	"strings"
	"testing"
)

func TestPath_StaysInTheDigitalDirectory(t *testing.T) {
	t.Parallel()

	got := Path("0b6f1b1e-3d5c-4a4f-9d0e-2b6a7c8d9e0f", "pdf")
	want := filepath.Join(Dir, "0b6f1b1e-3d5c-4a4f-9d0e-2b6a7c8d9e0f.pdf")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	// The name and ext are generated, but the join is what keeps a file from
	// being written outside the directory the app serves from.
	if filepath.Dir(got) != filepath.Clean(Dir) {
		t.Errorf("Path = %q, which is not in %s", got, Dir)
	}
}

// The name in the header is the one the operator uploaded, so it is the one
// place in this package that hostile text can reach. A quote or a newline read
// raw would end the header early and let the rest of the name be taken for
// headers of its own.
func TestDisposition_QuotesTheUploadName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"castellon.pdf",
		"a name with spaces.pdf",
		`quote"and\backslash.pdf`,
		"line\nbreak.pdf",
		"юникод.pdf",
	} {
		got := disposition(name)
		if strings.ContainsAny(got, "\n\r") {
			t.Errorf("disposition(%q) = %q, which carries a line break", name, got)
		}

		mediaType, params, err := mime.ParseMediaType(got)
		if err != nil {
			t.Errorf("disposition(%q) = %q, which does not parse: %v", name, got, err)
			continue
		}
		if mediaType != "attachment" {
			t.Errorf("disposition(%q) = %q, want an attachment", name, got)
		}
		if params["filename"] != name {
			t.Errorf("disposition(%q) names the file %q", name, params["filename"])
		}
	}
}
