package handlers

import (
	"checkout-api/store"
	"database/sql"
	"encoding/json"
	"fmt"
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

// func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
// 	// 1. Build the Xsolla API URL using your Project ID
// 	// Documentation: GET /v2/project/{project_id}/items/virtual_items
// 	url := fmt.Sprintf("https://store.xsolla.com/api/v2/project/%d/items/virtual_items", h.ProjectID)

// 	// 2. Create the request
// 	req, err := http.NewRequest("GET", url, nil)
// 	if err != nil {
// 		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create request"})
// 		return
// 	}

// 	// 3. (Optional) Add Language header if you want specific translations from Xsolla
// 	req.Header.Set("Accept", "application/json")

// 	// 4. Send the request to Xsolla
// 	client := &http.Client{Timeout: 10 * time.Second}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		h.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Xsolla API unreachable"})
// 		return
// 	}
// 	defer resp.Body.Close()

// 	// 5. Decode the response from Xsolla
// 	var result map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse Xsolla response"})
// 		return
// 	}

// 	// 6. Forward the result to your React frontend
// 	// Your React code is already set up to handle the "virtual_items" key!
// 	h.writeJSON(w, http.StatusOK, result)
// }

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
