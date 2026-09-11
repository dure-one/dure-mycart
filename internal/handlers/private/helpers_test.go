package handlers

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"testing"
)

func TestSniffMIMEType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		want    string
		wantErr bool
	}{
		{
			name:    "PNG image",
			content: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			want:    "image/png",
			wantErr: false,
		},
		{
			name:    "JPEG image",
			content: []byte{0xFF, 0xD8, 0xFF},
			want:    "image/jpeg",
			wantErr: false,
		},
		{
			name:    "plain text",
			content: []byte("hello world"),
			want:    "text/plain; charset=utf-8",
			wantErr: false,
		},
		{
			name:    "empty file",
			content: []byte{},
			want:    "text/plain; charset=utf-8",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create multipart file header
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", `form-data; name="file"; filename="test.dat"`)
			part, err := writer.CreatePart(header)
			if err != nil {
				t.Fatalf("create part: %v", err)
			}
			_, _ = part.Write(tt.content)
			_ = writer.Close()

			// Parse multipart to get FileHeader
			reader := multipart.NewReader(body, writer.Boundary())
			form, err := reader.ReadForm(10 << 20) // 10MB
			if err != nil {
				t.Fatalf("read form: %v", err)
			}
			defer func() { _ = form.RemoveAll() }()

			files := form.File["file"]
			if len(files) == 0 {
				t.Fatal("no file in form")
			}

			got, err := sniffMIMEType(files[0])
			if (err != nil) != tt.wantErr {
				t.Errorf("sniffMIMEType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sniffMIMEType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeExt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "uppercase extension",
			filename: "image.PNG",
			want:     "png",
		},
		{
			name:     "mixed case extension",
			filename: "document.PdF",
			want:     "pdf",
		},
		{
			name:     "already lowercase",
			filename: "file.txt",
			want:     "txt",
		},
		{
			name:     "multiple dots",
			filename: "archive.tar.gz",
			want:     "gz",
		},
		{
			name:     "no extension",
			filename: "README",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeExt(tt.filename); got != tt.want {
				t.Errorf("normalizeExt(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestGenerateFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		originalName string
		wantExt      string
	}{
		{
			name:         "image file",
			originalName: "photo.jpg",
			wantExt:      "jpg",
		},
		{
			name:         "uppercase extension",
			originalName: "document.PDF",
			wantExt:      "PDF",
		},
		{
			name:         "no extension",
			originalName: "README",
			wantExt:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uuid, ext, filename := generateFileName(tt.originalName)

			if len(uuid) == 0 {
				t.Error("generateFileName() uuid is empty")
			}
			if ext != tt.wantExt {
				t.Errorf("generateFileName() ext = %v, want %v", ext, tt.wantExt)
			}
			if len(filename) == 0 {
				t.Error("generateFileName() filename is empty")
			}
		})
	}
}
