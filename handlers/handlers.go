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

// 	// STEP 1: Handle the "Handshake" (User Validation)
// 	// Xsolla calls this the moment the user opens the payment UI
// 	if payload.NotificationType == "user_validation" {
// 		// You can check your DB here if the user exists,
// 		// but for now, we just tell Xsolla "Yes, this user is okay!"
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	// STEP 2: Handle the "Delivery" (Order Paid)
// 	// This is the actual combined webhook event
// 	// STEP 2: Handle the "Delivery" (Order Paid)
// 	if payload.NotificationType == "order_paid" {
// 		// 1. Try ExternalID, fallback to ID
// 		userID := payload.User.ExternalID
// 		if userID == "" {
// 			userID = payload.User.ID
// 		}

// 		// 2. Point to the nested items
// 		items := payload.Purchase.VirtualItems

// 		fmt.Printf("🔍 DEBUG: Webhook Type: %s\n", payload.NotificationType)
// 		fmt.Printf("🔍 DEBUG: User ID identified: [%s]\n", userID)
// 		fmt.Printf("🔍 DEBUG: Number of items found: %d\n", len(items))

// 		if userID == "" {
// 			fmt.Println("❌ ERROR: No User ID found in webhook")
// 			w.WriteHeader(http.StatusBadRequest)
// 			return
// 		}

// 		for _, item := range items {
// 			fmt.Printf("📦 Delivering SKU: %s (Qty: %d)\n", item.SKU, item.Quantity)
// 			err := h.Store.AddToInventory(r.Context(), userID, item.SKU, item.Quantity)
// 			if err != nil {
// 				fmt.Printf("❌ DB ERROR: %v\n", err)
// 				h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Failed to update inventory"})
// 				return
// 			}
// 		}

// 		// Success!
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}
// }

func (h *Handler) HandleXsollaWebhook(w http.ResponseWriter, r *http.Request) {
	fmt.Println("🚀 WEBHOOK RECEIVED! Checking payload...")
	var payload XsollaWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "Invalid payload"})
		return
	}

	// STEP 1: Handle User Validation
	if payload.NotificationType == "user_validation" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// STEP 2: Handle Order Paid — just acknowledge, Xsolla manages inventory
	if payload.NotificationType == "order_paid" {
		fmt.Printf("✅ order_paid received for user: %s\n", payload.User.ExternalID)
		// No DB write — Xsolla tracks inventory on their side.
		// Your GetInventory handler reads from Xsolla's API directly.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Ignore unrecognized events
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	url := fmt.Sprintf(
		"https://store.xsolla.com/api/v2/project/%d/user/inventory/items?limit=50&offset=0",
		h.ProjectID,
	)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", authHeader) // Forward the user's Bearer token

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to reach Xsolla", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	fmt.Printf("🔍 Xsolla inventory response: %s\n", string(body))
	w.Write(body)
}
