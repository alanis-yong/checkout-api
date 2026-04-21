package handlers

import (
	"checkout-api/store"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Handler holds the configuration for the entire API
type Handler struct {
	MerchantID string
	APIKey     string
	ProjectID  int
	Store      *store.Queries
}

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type XsollaWebhook struct {
	NotificationType string `json:"notification_type"`
	User             struct {
		// Xsolla uses "external_id" for the ID you provided
		ExternalID string `json:"external_id"`
	} `json:"user"`
	Items []struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
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

	// GET https://login.xsolla.com/api/users/me
	// This is the classic endpoint to verify a user's JWT
	req, _ := http.NewRequest("GET", "https://login.xsolla.com/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// In your handler.go or main.go
func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
	// 1. Load the items from your JSON file
	data, err := os.ReadFile("virtual-items.json")
	if err != nil {
		// Fallback to empty if file is missing
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"virtual_items": []interface{}{}})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "JSON parse error"})
		return
	}

	// 2. Send it back. React is looking for the "virtual_items" key!
	h.writeJSON(w, http.StatusOK, result)
}

func (h *Handler) HandleXsollaWebhook(w http.ResponseWriter, r *http.Request) {
	fmt.Println("🚀 WEBHOOK RECEIVED! Checking payload...")
	var payload XsollaWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "Invalid payload"})
		return
	}

	// STEP 1: Handle the "Handshake" (User Validation)
	// Xsolla calls this the moment the user opens the payment UI
	if payload.NotificationType == "user_validation" {
		// You can check your DB here if the user exists,
		// but for now, we just tell Xsolla "Yes, this user is okay!"
		w.WriteHeader(http.StatusOK)
		return
	}

	// STEP 2: Handle the "Delivery" (Order Paid)
	// This is the actual combined webhook event
	if payload.NotificationType == "order_paid" {
		userID := payload.User.ExternalID

		fmt.Printf("🔍 DEBUG: Webhook Type: %s\n", payload.NotificationType)
		fmt.Printf("🔍 DEBUG: Raw User ID from JSON: [%s]\n", userID)
		fmt.Printf("🔍 DEBUG: Number of items found: %d\n", len(payload.Items))

		for _, item := range payload.Items {
			// Your existing database logic - this part is perfect!
			err := h.Store.AddToInventory(r.Context(), userID, item.SKU, item.Quantity)
			if err != nil {
				h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Failed to update inventory"})
				return
			}
		}
		// Success! Xsolla likes 204 No Content for successful deliveries
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// If we get an event we don't recognize, just ignore it
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	// For now, we use a hardcoded user, but you can get this from the token later
	userID := "user_alanis_01"

	inventory, err := h.Store.GetInventory(r.Context(), userID)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Could not fetch inventory"})
		return
	}

	h.writeJSON(w, http.StatusOK, inventory)
}
