package fsutil

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every helper here creates directories the caller did not ask about, so the
// interesting behaviour is what happens when it cannot: the error has to come
// back instead of a half-written file.

func TestMkDirs_RefusesAPathUnderAFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("create blocker: %v", err)
	}

	if err := MkDirs(0o755, filepath.Join(blocker, "under")); err == nil {
		t.Error("MkDirs reported success for a path under a regular file")
	}
}

func TestMkSubDirs_RefusesAPathUnderAFile(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatalf("create blocker: %v", err)
	}

	if err := MkSubDirs(0o755, parent, "blocker"); err == nil {
		t.Error("MkSubDirs reported success although the name is taken by a file")
	}
}

func TestOpenFile_Errors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	t.Run("parent is a file", func(t *testing.T) {
		t.Parallel()

		blocker := filepath.Join(root, "file")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatalf("create blocker: %v", err)
		}

		f, err := OpenFile(filepath.Join(blocker, "child"), FsCWFlags, 0o644)
		if err == nil {
			_ = f.Close()
			t.Error("OpenFile opened a file inside a regular file")
		}
	})

	t.Run("the path is a directory", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(root, "dir")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create dir: %v", err)
		}

		f, err := OpenFile(dir, FsCWFlags, 0o644)
		if err == nil {
			_ = f.Close()
			t.Error("OpenFile opened a directory for writing")
		}
	})
}

// WriteOSFile accepts three data types and panics on anything else. The panic is
// deliberate — it is a programming error, not a runtime one — and closing the
// file before panicking is what keeps the caller from leaking a descriptor.
func TestWriteOSFile_PanicsOnAnUnsupportedType(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "out")
	f, err := OpenFile(path, FsCWTFlags, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("WriteOSFile did not panic on an unsupported data type")
		}
		if msg, ok := recovered.(string); !ok || !strings.Contains(msg, "data type only allow") {
			t.Errorf("unexpected panic value: %v", recovered)
		}
		if err := f.Close(); err == nil {
			t.Error("the file was left open by the panic path")
		}
	}()

	_, _ = WriteOSFile(f, 42)
}

func TestWriteOSFile_FromAReader(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "out")
	f, err := OpenFile(path, FsCWTFlags, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}

	n, err := WriteOSFile(f, strings.NewReader("from a reader"))
	if err != nil {
		t.Fatalf("WriteOSFile: %v", err)
	}
	if n != len("from a reader") {
		t.Errorf("wrote %d bytes, want %d", n, len("from a reader"))
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "from a reader" {
		t.Errorf("file holds %q", got)
	}
}

// An embedded filesystem with nothing in it, and a folder name that matches
// nothing, are both non-errors: there is simply nothing to extract, and the
// working directory has to be left alone. (The signature takes a concrete
// embed.FS, so a broken filesystem — the branch that would report a read
// failure — cannot be constructed from a test.)
func TestEmbedExtract_ExtractsNothing(t *testing.T) {
	var empty embed.FS

	tests := []struct {
		name   string
		fsys   embed.FS
		folder string
	}{
		{"empty filesystem", empty, "testdata"},
		{"no matching folder", embeddedSample, "does-not-exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			if err := EmbedExtract(tt.fsys, tt.folder); err != nil {
				t.Fatalf("EmbedExtract: %v", err)
			}
			entries, err := os.ReadDir(".")
			if err != nil {
				t.Fatalf("read the working directory: %v", err)
			}
			for _, e := range entries {
				t.Errorf("EmbedExtract wrote %s although there was nothing to extract", e.Name())
			}
		})
	}
}
