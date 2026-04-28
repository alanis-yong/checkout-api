package handlers

import (
	"encoding/json"
	"net/http"
)

// GetCart - Retrieves all items in the cart for a specific user
func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query("SELECT sku, quantity FROM cart WHERE user_id = $1", userID)
	if err != nil {
		http.Error(w, "DB Error: "+err.Error(), 500)
		return
	}
	defer rows.Close()

	type CartResult struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	}

	var items []CartResult
	for rows.Next() {
		var i CartResult
		if err := rows.Scan(&i.SKU, &i.Quantity); err == nil {
			items = append(items, i)
		}
	}

	// If no items found, return an empty array [] instead of null
	if items == nil {
		items = []CartResult{}
	}

	h.writeJSON(w, http.StatusOK, items)
}

// AddToCart - Upserts a record: if it exists, increment. If not, create.
func (h *Handler) AddToCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		SKU    string `json:"sku"`
		Delta  int    `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", 400)
		return
	}

	query := `
       INSERT INTO cart (user_id, sku, quantity)
       VALUES ($1, $2, 1)
       ON CONFLICT (user_id, sku)
       DO UPDATE SET quantity = cart.quantity + 1`

	_, err := h.DB.Exec(query, req.UserID, req.SKU)
	if err != nil {
		http.Error(w, "DB Error: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// UpdateCartQuantity - Increments or decrements quantity, removes if <= 0
func (h *Handler) UpdateCartQuantity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		SKU    string `json:"sku"`
		Delta  int    `json:"delta"` // Expects +1 or -1
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	_, err := h.DB.Exec(`
       UPDATE cart
       SET quantity = quantity + $1
       WHERE user_id = $2 AND sku = $3`, req.Delta, req.UserID, req.SKU)

	if err != nil {
		http.Error(w, "DB Error: "+err.Error(), 500)
		return
	}

	// Clean up: If an item hits 0 or less, remove it from the table
	h.DB.Exec("DELETE FROM cart WHERE quantity <= 0")

	w.WriteHeader(http.StatusOK)
}

// ClearCart - Removes all items for a user
func (h *Handler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	_, err := h.DB.Exec("DELETE FROM cart WHERE user_id = $1", userID)
	if err != nil {
		http.Error(w, "DB Error", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}
