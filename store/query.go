package store

import (
	"context"
	"database/sql"
)

// 1. DATA STRUCTURES
type Product struct {
	ID            int     `json:"id"`
	SKU           string  `json:"sku"`
	NameEn        string  `json:"name_en"`
	NameCn        string  `json:"name_cn"`
	PriceUsd      float64 `json:"price_usd"`
	PriceMyr      float64 `json:"price_myr"`
	PurchaseLimit int     `json:"purchase_limit"`
}

type InventoryItem struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// 2. SQL CONSTANTS
const (
	GetProductsQuery = `SELECT id, sku, name_en, name_cn, price_usd, price_myr, purchase_limit FROM products`

	GetInventoryQuery = `SELECT sku, quantity FROM inventory WHERE user_id = $1`

	AddToInventoryQuery = `
    INSERT INTO inventory (user_id, sku, quantity) 
    VALUES ($1, $2, $3)
    ON CONFLICT (user_id, sku) 
    DO UPDATE SET quantity = inventory.quantity + EXCLUDED.quantity`
)

// 3. DATABASE SETUP
type DBTX interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) // Required for INSERT/UPDATE
}

type Queries struct {
	db DBTX
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}
