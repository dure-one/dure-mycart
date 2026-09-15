// Package digitalfiles owns the shop's digital goods: where their bytes live on
// disk, and how one of them is handed to a client.
//
// A digital good is stored as a file named after a UUID and handed to whoever is
// entitled to it: an admin downloading it from the panel, a buyer downloading it
// from the cabinet, and the purchase letter, which attaches it. All three read
// the same bytes from the same place, so where those bytes live and how a stored
// name becomes a path are decided here rather than beside any one caller.
package digitalfiles

import (
	"bytes"
	"mime"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
)

// Dir is where the shop's digital goods are kept, relative to the process's
// working directory. The process creates it at startup (see requiredDirs in
// internal/init.go), so a shop that has not uploaded anything yet can still
// write here.
const Dir = "./lc_digitals"

// Path is where the bytes of a stored digital file live. It is built from the
// name the shop chose and the extension it recorded, never from the name the
// operator uploaded, so the value cannot name a file outside Dir.
func Path(name, ext string) string {
	return filepath.Join(Dir, name+"."+ext)
}

// Serve hands a stored digital file to the client as a download.
//
// The bytes are read into memory rather than streamed: fasthttp consumes a
// response body stream only after the handler has returned, so an *os.File
// handed to SendStream would be closed by the time it is read. The framework's
// own streaming primitives (SendFile, Download) would stream, but they take over
// the two headers below — the content type would become whatever the extension
// suggests and the disposition would be re-encoded — and serving an operator's
// upload as anything but an unrendered attachment is the one thing this function
// is here to prevent. A guide is a few megabytes, so the copy is the cheaper
// side of that trade.
//
// It returns the read error as it came: a row whose bytes are missing is the
// shop's problem to notice, and each caller decides what to tell the client —
// both currently answer 404, which is also what a file that never existed
// answers.
func Serve(c fiber.Ctx, file *models.File) error {
	content, err := os.ReadFile(Path(file.Name, file.Ext))
	if err != nil {
		return err
	}

	// Never the browser's guess: what is stored here may be anything the
	// operator sells, and it must be saved rather than rendered.
	c.Set(fiber.HeaderContentType, "application/octet-stream")
	c.Set(fiber.HeaderContentDisposition, disposition(file.OrigName))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")

	return c.SendStream(bytes.NewReader(content))
}

// disposition is the Content-Disposition header naming the file the buyer will
// find on disk. The name is the one the operator uploaded, so it is quoted and
// encoded by mime rather than interpolated: a name carrying a quote, a newline
// or anything outside ASCII would otherwise end the header early and let the
// rest of the name be read as headers of its own.
func disposition(name string) string {
	if value := mime.FormatMediaType("attachment", map[string]string{"filename": name}); value != "" {
		return value
	}
	return "attachment"
}
