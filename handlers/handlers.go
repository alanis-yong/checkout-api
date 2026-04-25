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
		ID         string `json:"id"`          // Xsolla's internal ID
		ExternalID string `json:"external_id"` // This is YOUR User ID (from the token)
	} `json:"user"`
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

// func (h *Handler) HandleXsollaWebhook(w http.ResponseWriter, r *http.Request) {
// 	fmt.Println("🚀 WEBHOOK RECEIVED! Checking payload...")
// 	var payload XsollaWebhook
// 	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
// 		h.writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "Invalid payload"})
// 		return
// 	}

// 	if payload.NotificationType == "user_validation" {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	if payload.NotificationType == "order_paid" {
// 		userID := payload.User.ExternalID
// 		if userID == "" {
// 			userID = payload.User.ID
// 		}

// 		if userID == "" {
// 			fmt.Println("❌ ERROR: No User ID found in webhook")
// 			w.WriteHeader(http.StatusBadRequest)
// 			return
// 		}

// 		items := payload.Purchase.VirtualItems
// 		fmt.Printf("✅ order_paid received for user: %s — delivering %d items\n", userID, len(items))

// 		for _, it := range items {
// 			fmt.Printf("📦 Delivering SKU: %s (Qty: %d) to user %s\n", it.SKU, it.Quantity, userID)
// 			if h.Store != nil {
// 				if err := h.Store.AddToInventory(r.Context(), userID, it.SKU, it.Quantity); err != nil {
// 					fmt.Printf("❌ DB ERROR adding to inventory: %v\n", err)
// 					h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Failed to update inventory"})
// 					return
// 				}
// 			}
// 		}

// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}

// 	w.WriteHeader(http.StatusNoContent)
// }

func (h *Handler) HandleXsollaWebhook(w http.ResponseWriter, r *http.Request) {
	fmt.Println("🚀 WEBHOOK RECEIVED! Checking payload...")

	// 1. Read the raw body to see EXACTLY what Xsolla is sending
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println("❌ Could not read request body")
		return
	}
	fmt.Printf("Raw JSON from Xsolla: %s\n", string(body))

	// 2. Decode the JSON
	var payload XsollaWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("❌ JSON Decode Error: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Printf("Parsed Notification: [%s]\n", payload.NotificationType)
	fmt.Printf("Parsed ExternalID: [%s]\n", payload.User.ExternalID)

	// 3. Check for Order Paid
	if payload.NotificationType == "order_paid" || payload.NotificationType == "payment" {
		fmt.Println("✅ Found a successful payment event!")

		userID := payload.User.ExternalID
		if userID == "" {
			userID = payload.User.ID
		}

		items := payload.Purchase.VirtualItems
		fmt.Printf("📦 User %s bought %d items\n", userID, len(items))

		// This is where you decide to save to DB or not
		// For now, just print to see if we reached it!
		for _, it := range items {
			fmt.Printf("👉 Item: %s, Qty: %d\n", it.SKU, it.Quantity)
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
