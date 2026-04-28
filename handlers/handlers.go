package handlers

import (
	"checkout-api/store"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Handler holds the configuration for the entire API
type Handler struct {
	MerchantID string
	APIKey     string
	ProjectID  int
	Store      *store.Queries
	DB         *sql.DB // <--- MAKE SURE THIS IS HERE
}

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}
type XsollaWebhook struct {
	NotificationType string `json:"notification_type"`
	User             struct {
		ID         string `json:"id"`
		ExternalID string `json:"external_id"`
	} `json:"user"`

	// Xsolla sometimes sends items at the top level
	Items []struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`

	// BUT they usually send them here for successful payments
	Purchase struct {
		VirtualItems []struct {
			SKU      string `json:"sku"`
			Quantity int    `json:"quantity"`
		} `json:"virtual_items"`
	} `json:"purchase"`
}

// NewHandler initializes a clean handler
func NewHandler() *Handler {
	return &Handler{}
}

// ErrorResponse is a standard structure for returning errors
type ErrorResponse struct {
	Message string `json:"message"`
}

// writeJSON is a helper to ensure all responses are valid JSON
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) VerifyXsollaToken(token string) (bool, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", "https://login.xsolla.com/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	locale := "en"
	if lang == "cn" {
		locale = "cn" // Changed from "zh" to "cn"
	}

	// Pass the locale directly to Xsolla
	url := fmt.Sprintf("https://store.xsolla.com/api/v2/project/%d/items/virtual_items?locale=%s", h.ProjectID, locale)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Request failed"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		h.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Xsolla unreachable"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	h.writeJSON(w, http.StatusOK, result)
}

func (h *Handler) HandleXsollaWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload XsollaWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Use ExternalID if available, otherwise fallback to ID
	userID := payload.User.ExternalID
	if userID == "" {
		userID = payload.User.ID
	}

	if payload.NotificationType == "order_paid" || payload.NotificationType == "payment" {
		// Determine which item list to use
		items := payload.Items
		if len(items) == 0 {
			items = payload.Purchase.VirtualItems
		}

		fmt.Printf("📦 Processing %d items for User: %s\n", len(items), userID)

		for _, it := range items {
			// 🚀 THE KEY CONNECTION: Save to your DB
			// This ensures your local inventory is always in sync with Xsolla
			err := h.Store.AddUserInventory(r.Context(), userID, it.SKU, it.Quantity)
			if err != nil {
				fmt.Printf("❌ Failed to update local inventory for SKU %s: %v\n", it.SKU, err)
				// We still return 204 to Xsolla so they don't spam us with retries
			} else {
				fmt.Printf("✅ Successfully added %d of %s to user inventory\n", it.Quantity, it.SKU)
			}
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	// 1. Get the user_id from the URL query
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id query parameter is required"})
		return
	}

	// 2. Fetch from your local PostgreSQL (using the Store method we created)
	inventory, err := h.Store.GetInventory(r.Context(), userID)
	if err != nil {
		fmt.Printf("❌ Database error fetching inventory: %v\n", err)
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not retrieve inventory"})
		return
	}

	// 3. Return the items to your React frontend
	h.writeJSON(w, http.StatusOK, inventory)
}
