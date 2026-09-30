package imageutil

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// Open decodes an image file (PNG or JPEG only).
// Returns the decoded image or an error if the file is not a valid PNG/JPEG.
func Open(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Only accept PNG and JPEG
	if format != "png" && format != "jpeg" {
		return nil, fmt.Errorf("unsupported format: %s (only PNG and JPEG allowed)", format)
	}

	return img, nil
}

// Fill resizes and crops an image to fill the target dimensions.
// Uses Catmull-Rom interpolation (similar to Lanczos) for high-quality resizing.
func Fill(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// Calculate scale to fill the target dimensions
	scaleX := float64(width) / float64(srcWidth)
	scaleY := float64(height) / float64(srcHeight)
	scale := scaleX
	if scaleY > scaleX {
		scale = scaleY
	}

	// Calculate scaled dimensions
	scaledWidth := int(float64(srcWidth) * scale)
	scaledHeight := int(float64(srcHeight) * scale)

	// Create scaled image
	scaledImg := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
	draw.CatmullRom.Scale(scaledImg, scaledImg.Bounds(), src, srcBounds, draw.Over, nil)

	// Crop to target dimensions (center)
	offsetX := (scaledWidth - width) / 2
	offsetY := (scaledHeight - height) / 2
	cropRect := image.Rect(offsetX, offsetY, offsetX+width, offsetY+height)

	croppedImg := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(croppedImg, croppedImg.Bounds(), scaledImg, cropRect.Min, draw.Src)

	return croppedImg
}

// Save encodes and saves an image to a file.
// Format is determined by the file extension (.png or .jpg/.jpeg).
func Save(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return png.Encode(f, img)
	case ".jpg", ".jpeg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	default:
		return fmt.Errorf("unsupported file extension: %s", ext)
	}
}
