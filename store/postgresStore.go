package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type XsollaFile struct {
	VirtualItems []XsollaItem `json:"virtual_items"`
}

type XsollaItem struct {
	SKU  string `json:"sku"`
	Name struct {
		En string `json:"en"`
		Zh string `json:"zh"`
	} `json:"name"`
	// This is the most sensitive part:
	Prices []struct {
		Amount   float64 `json:"amount"`   // Must be lowercase
		Currency string  `json:"currency"` // Must be lowercase
	} `json:"prices"`
}

// GetProducts executes the constant GetProductsQuery
func (q *Queries) GetProducts(ctx context.Context) ([]Product, error) {
	rows, err := q.db.QueryContext(ctx, GetProductsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Product
	for rows.Next() {
		var i Product
		if err := rows.Scan(&i.ID, &i.SKU, &i.NameEn, &i.NameCn, &i.PriceUsd, &i.PriceMyr, &i.PurchaseLimit); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// AddToInventory executes the constant AddToInventoryQuery
func (q *Queries) AddToInventory(ctx context.Context, userID string, sku string, qty int) error {
	_, err := q.db.ExecContext(ctx, AddToInventoryQuery, userID, sku, qty)
	return err
}

// GetInventory executes the constant GetInventoryQuery
func (q *Queries) GetInventory(ctx context.Context, userID string) ([]InventoryItem, error) {
	rows, err := q.db.QueryContext(ctx, GetInventoryQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InventoryItem
	for rows.Next() {
		var i InventoryItem
		if err := rows.Scan(&i.SKU, &i.Quantity); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

func (q *Queries) SeedDatabase(ctx context.Context) error {
	// 1. CLEAR THE STALE DATA FIRST
	_, err := q.db.ExecContext(ctx, "TRUNCATE TABLE products CASCADE;")
	if err != nil {
		log.Printf("Could not truncate products: %v", err)
	} else {
		log.Println("🗑️ Database wiped for fresh seeding")
	}

	content, err := os.ReadFile("virtual-items.json")
	if err != nil {
		return fmt.Errorf("could not read virtual-items.json: %w", err)
	}

	var fileData XsollaFile
	if err := json.Unmarshal(content, &fileData); err != nil {
		return fmt.Errorf("failed to parse xsolla json: %w", err)
	}

	for _, item := range fileData.VirtualItems {
		nameEn := item.Name.En
		if nameEn == "" {
			nameEn = item.SKU
		}

		var usd float64
		if len(item.Prices) > 0 {
			usd = item.Prices[0].Amount
		}

		// FORCED FALLBACK: If JSON price is missing/zero, use 10.00
		if usd <= 0 {
			usd = 10.00
		}

		// 2. INSERT FRESH DATA
		_, err := q.db.ExecContext(ctx, `
     INSERT INTO products (sku, name_en, name_cn, price_usd, price_myr, purchase_limit)
     VALUES ($1, $2, $3, $4, $5, $6)`,
			item.SKU, nameEn, nameEn, usd, usd*4.5, 10,
		)
		if err != nil {
			log.Printf("Warning: failed to seed item %s: %v", item.SKU, err)
		}
	}

	log.Println("✅ Database successfully synced with fresh prices")
	return nil
}
