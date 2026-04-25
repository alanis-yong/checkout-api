package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TokenRequest matches the data coming from your React frontend
type TokenRequest struct {
	UserID   string  `json:"user_id"`
	Email    string  `json:"email"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Language string  `json:"language"`
	Items    []struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
}

func (h *Handler) GetXsollaToken(w http.ResponseWriter, r *http.Request) {
	idempotency := r.Header.Get("Idempotency-Key")
	if idempotency == "" {
		h.writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "Missing Idempotency-Key"})
		return
	}

	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, ErrorResponse{Message: "Invalid request body"})
		return
	}

	fmt.Printf("Token Request for User: [%s], Email: [%s]\n", req.UserID, req.Email)

	xsollaPayload := map[string]interface{}{
		"user": map[string]interface{}{
			"id":      map[string]interface{}{"value": req.UserID},
			"email":   map[string]interface{}{"value": req.Email},
			"country": map[string]interface{}{"value": "USD"},
		},
		"purchase": map[string]interface{}{
			"virtual_items": map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"sku":    "EQUIP_SHIELD_GOLD_01",
						"amount": 9.99,
					},
				},
			},
		},
		"settings": map[string]interface{}{
			"project_id": h.ProjectID,
			"mode":       "sandbox",
			"currency":   "USD",
		},
	}

	body, _ := json.Marshal(xsollaPayload)
	url := fmt.Sprintf("https://api.xsolla.com/merchant/v2/merchants/%s/token", h.MerchantID)

	xReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	xReq.Header.Set("Content-Type", "application/json")
	xReq.SetBasicAuth(h.MerchantID, h.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(xReq)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, ErrorResponse{Message: "Xsolla unreachable"})
		return
	}
	defer resp.Body.Close()

	xsollaResponseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Xsolla API Error (%d): %s\n", resp.StatusCode, string(xsollaResponseBody))
	}

	var xResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&xResult)
	h.writeJSON(w, http.StatusOK, xResult)
}

// XsollaWebhook receives the notification from Xsolla after a user pays
func (h *Handler) XsollaWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData map[string]interface{}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// This is the "Entitlement" part of your flow
	notificationType, ok := webhookData["notification_type"].(string)
	if ok && notificationType == "payment" {
		fmt.Printf("💰 Entitlement Triggered! User %v successfully paid.\n", webhookData["user"])
		// In a real app, you'd update your DB here
	}

	// Xsolla requires a 204 No Content or 200 OK to stop retrying the webhook
	w.WriteHeader(http.StatusNoContent)
}
