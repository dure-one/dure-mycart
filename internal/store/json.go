package queries

import (
	"encoding/json"
	"fmt"

	"github.com/shurco/mycart/internal/models"
)

// decodeJSONArray decodes an aggregated JSON array into dst, dropping entries
// whose id decodes as empty.
//
// The id column is a primary key, so a real row always has one; skipping empty
// ids keeps a null placeholder entry — if an engine ever emits one — out of the
// slice as an unusable element.
func decodeJSONArray[T any](raw string, dst *[]T, id func(T) string) error {
	if raw == "" || raw == "[]" {
		return nil
	}

	var items []T
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return fmt.Errorf("decode json array: %w", err)
	}

	for _, item := range items {
		if id(item) == "" {
			continue
		}
		*dst = append(*dst, item)
	}
	return nil
}

// decodeFiles decodes the product image aggregate.
func decodeFiles(raw string, dst *[]models.File) error {
	return decodeJSONArray(raw, dst, func(f models.File) string { return f.ID })
}

// decodeVariants decodes the product variant aggregate.
func decodeVariants(raw string, dst *[]models.ProductVariant) error {
	return decodeJSONArray(raw, dst, func(v models.ProductVariant) string { return v.ID })
}
