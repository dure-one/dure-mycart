package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/security"
)

func main() {
	// Initialize database. The same resolution order as the CLI applies:
	// flags are absent here, so the environment and lc_base/config.json win.
	cfg, err := database.Resolve(database.Overrides{})
	if err != nil {
		log.Fatalf("Failed to resolve database configuration: %v", err)
	}
	if err := queries.New(cfg, migrations.Embed()); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Read CSV file
	file, err := os.Open("data/sample_products.csv")
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}

	ctx := context.Background()
	count := 0

	// Skip header row
	for i, record := range records[1:] {
		if len(record) < 8 {
			log.Printf("Skipping invalid row %d", i+2)
			continue
		}

		amount, _ := strconv.Atoi(record[4])
		quantity, _ := strconv.Atoi(record[5])
		active := record[7] == "true"

		product := &models.Product{
			Core:        models.Core{ID: security.RandomString()},
			Name:        record[0],
			Slug:        record[1],
			Brief:       record[2],
			Description:record[3],
			Amount:      amount,
			Quantity:    quantity,
			SKU:         record[6],
			Active:      active,
			Metadata:    []models.Metadata{},
			Attributes:  []string{"Sample", "Generated"},
			Digital:     models.Digital{},
		}

		if _, err := store.AddProductWithVariants(ctx, product); err != nil {
			log.Printf("Failed to create product %s: %v", product.Name, err)
			continue
		}

		count++
		if count%20 == 0 {
			fmt.Printf("Created %d products...\n", count)
		}
	}

	fmt.Printf("✓ Successfully created %d sample products!\n", count)
}
