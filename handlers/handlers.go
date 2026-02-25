package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"checkout-api/models"
)

// ItemStore defines the data operations the handler needs.
type ItemStore interface {
	GetItems() []*models.Item
	GetItem(id int) *models.Item
	CreateOrder(userID int, items []models.LineItem, total int, status string) *models.Order
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store ItemStore
}

// NewHandler creates a Handler with the given store.
func NewHandler(s ItemStore) *Handler {
	return &Handler{store: s}
}

// CreateOrderRequest is the payload for POST /orders.
type CreateOrderRequest struct {
	UserID int `json:"user_id"`
	Items  []struct {
		ItemID   int `json:"item_id"`
		Quantity int `json:"quantity"`
	} `json:"items"`
}

// PaymentResult represents a response from the payment provider.
type PaymentResult struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// mockProcessPayment simulates a payment provider call.
func mockProcessPayment(amount int) PaymentResult {
	if amount > 0 && amount < 1000000 {
		return PaymentResult{
			Success:       true,
			TransactionID: fmt.Sprintf("txn_%d", time.Now().UnixNano()),
		}
	}
	return PaymentResult{
		Success: false,
		Error:   "Payment declined",
	}
}

// GetItems handles GET /items — returns all available items.
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	items := h.store.GetItems()
	writeJSON(w, http.StatusOK, items)
}

// GetItemByID handles GET /items/{id} — returns a single item.
func (h *Handler) GetItemByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/items/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	item := h.store.GetItem(id)
	if item == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// CreateOrder handles POST /orders — creates an order with mock payment.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and calculate total
	total := 0
	orderItems := make([]models.LineItem, 0, len(req.Items))
	for _, item := range req.Items {
		storeItem := h.store.GetItem(item.ItemID)
		if storeItem == nil {
			http.Error(w, fmt.Sprintf("Item %d not found", item.ItemID), http.StatusBadRequest)
			return
		}
		itemTotal := storeItem.Price * item.Quantity
		total += itemTotal
		orderItems = append(orderItems, models.LineItem{
			ItemID:   item.ItemID,
			Quantity: item.Quantity,
			Price:    storeItem.Price,
		})
	}

	// Process payment (mock)
	paymentResult := mockProcessPayment(total)

	status := "paid"
	if !paymentResult.Success {
		status = "failed"
	}

	order := h.store.CreateOrder(req.UserID, orderItems, total, status)

	if paymentResult.Success {
		writeJSON(w, http.StatusCreated, map[string]any{
			"order":   order,
			"payment": paymentResult,
		})
	} else {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"order":   order,
			"payment": paymentResult,
		})
	}
}

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
