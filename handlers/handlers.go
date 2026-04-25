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

	// Xsolla sends items here for order_paid!
	Items []struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`

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
	fmt.Println("🚀 WEBHOOK RECEIVED! Checking payload...")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println("❌ Could not read request body")
		return
	}
	fmt.Printf("Raw JSON from Xsolla: %s\n", string(body))

	var payload XsollaWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("❌ JSON Decode Error: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	userID := payload.User.ExternalID
	if userID == "" {
		userID = payload.User.ID

	fmt.Printf("Parsed Notification: [%s]\n", payload.NotificationType)
	fmt.Printf("User ID found: [%s]\n", userID)

	if payload.NotificationType == "order_paid" || payload.NotificationType == "payment" {

		items := payload.Items
		if len(items) == 0 {
			items = payload.Purchase.VirtualItems
		}

		fmt.Printf("📦 Found %d items to deliver\n", len(items))

		if len(items) > 0 {
			for _, it := range items {
				fmt.Printf("👉 Delivering SKU: %s, Qty: %d to User %s\n", it.SKU, it.Quantity, userID)
				// If you want to save to your local DB, add h.Store.AddToInventory here
			}
		} else {
			fmt.Println("ℹ️ No items found in this notification (typical for basic 'payment' types).")
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	fmt.Println("ℹ️ Webhook was not order_paid, acknowledging and exiting.")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.writeJSON(w, http.StatusUnauthorized, ErrorResponse{Message: "Missing Authorization header"})
		return
	}

	url := fmt.Sprintf(
		"https://store.xsolla.com/api/v2/project/%d/user/inventory/items?limit=50&offset=0&sandbox=1",
		h.ProjectID,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Failed to build request"})
		return
	}

	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Xsolla Request Error: %v\n", err)
		h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Failed to reach Xsolla"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
