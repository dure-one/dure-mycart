//go:build tools
// +build tools

package main

import (
	"image"
	"image/color"
	"image/draw"
	"log"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func main() {
	// Create 400x400 image with light gray background
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	gray := color.RGBA{243, 244, 246, 255} // #F3F4F6
	draw.Draw(img, img.Bounds(), &image.Uniform{gray}, image.Point{}, draw.Src)

	// Add "No Image" text
	textColor := color.RGBA{107, 114, 128, 255} // #6B7280
	point := fixed.Point26_6{
		X: fixed.I(150),
		Y: fixed.I(210),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textColor),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString("No Image")

	// Save to file
	err := imaging.Save(img, "./cmd/lc_uploads/product-placeholder.png")
	if err != nil {
		log.Fatalf("Failed to save placeholder: %v", err)
	}

	log.Println("Placeholder created: ./cmd/lc_uploads/product-placeholder.png")
}
