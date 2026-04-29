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
	// Inside GetXsollaToken
	formattedItems := make([]map[string]interface{}, len(req.Items))
	for i, item := range req.Items {
		formattedItems[i] = map[string]interface{}{
			"sku":      item.SKU,
			"quantity": item.Quantity, // Use 'quantity' to match the Admin API spec
		}
	}

	xsollaPayload := map[string]interface{}{
		"user": map[string]interface{}{
			"id": map[string]interface{}{
				"value": req.UserID,
			},
			"name": map[string]interface{}{
				"value": req.UserID, // Using ID as name since we don't have a name field yet
			},
			"email": map[string]interface{}{
				"value": req.Email,
			},
			"country": map[string]interface{}{
				"value":        "US",
				"allow_modify": false,
			},
		},
		"purchase": map[string]interface{}{
			"items": formattedItems,
		},
		"settings": map[string]interface{}{
			"language":    "en",
			"external_id": idempotency,
			"mode":        "sandbox",
			"currency":    req.Currency,
			"return_url":  "https://xsolla-alanis-gamestore.vercel.app/store",
			"ui": map[string]interface{}{
				"theme": "63295aab2e47fab76f7708e3",
			},
		},
	}

	body, _ := json.Marshal(xsollaPayload)
	url := fmt.Sprintf("https://store.xsolla.com/api/v3/project/%d/admin/payment/token", h.ProjectID)

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

	xsollaResponseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read Xsolla response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Xsolla Error (%d): %s\n", resp.StatusCode, string(xsollaResponseBody))
		h.writeJSON(w, resp.StatusCode, map[string]string{"error": "Xsolla rejected request"})
		return
	}

	var xResult map[string]interface{}
	if err := json.Unmarshal(xsollaResponseBody, &xResult); err != nil {
		fmt.Printf("❌ JSON Parse Error: %v\n", err)
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Invalid JSON from Xsolla"})
		return
	}

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
