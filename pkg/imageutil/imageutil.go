package imageutil

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

var (
	// ErrUnsupportedFormat is returned when trying to decode an unsupported image format.
	ErrUnsupportedFormat = errors.New("unsupported image format")
)

// Open decodes an image from the file at path.
// Only PNG and JPEG formats are supported; all others return ErrUnsupportedFormat.
// This explicitly prevents TIFF and other formats from being processed.
func Open(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Peek at format without decoding the whole image
	_, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}

	// Only allow PNG and JPEG
	if format != "png" && format != "jpeg" {
		return nil, fmt.Errorf("%w: %s (only PNG and JPEG allowed)", ErrUnsupportedFormat, format)
	}

	// Rewind and decode
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	img, _, err := image.Decode(f)
	return img, err
}

// Fill resizes and crops the image to fit the specified width and height.
// Uses Catmull-Rom interpolation for high-quality resizing.
func Fill(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()

	// Calculate scaling to fill the target dimensions
	scaleX := float64(width) / float64(srcW)
	scaleY := float64(height) / float64(srcH)
	scale := scaleX
	if scaleY > scaleX {
		scale = scaleY
	}

	// Scaled dimensions
	scaledW := int(float64(srcW) * scale)
	scaledH := int(float64(srcH) * scale)

	// Create scaled image
	scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, srcBounds, draw.Over, nil)

	// Crop to center
	cropX := (scaledW - width) / 2
	cropY := (scaledH - height) / 2
	cropRect := image.Rect(cropX, cropY, cropX+width, cropY+height)

	cropped := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(cropped, cropped.Bounds(), scaled, cropRect.Min, draw.Src)

	return cropped
}

// Save encodes the image to the file at path.
// Format is determined by file extension: .png for PNG, .jpg/.jpeg for JPEG.
// All other extensions return an error.
func Save(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return png.Encode(f, img)
	case ".jpg", ".jpeg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	default:
		return fmt.Errorf("unsupported output format: %s (only .png, .jpg, .jpeg allowed)", ext)
	}
}
