package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"checkout-api/store"
)

func TestGetItems(t *testing.T) {
	s := store.NewStore()
	h := NewHandler(s)

	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "GET returns items",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			wantBody:   "Laptop",
		},
		{
			name:       "POST not allowed",
			method:     http.MethodPost,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/items", nil)
			w := httptest.NewRecorder()

			h.GetItems(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body missing %q, got: %s", tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestGetItemByID(t *testing.T) {
	s := store.NewStore()
	h := NewHandler(s)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid item",
			method:     http.MethodGet,
			path:       "/items/1",
			wantStatus: http.StatusOK,
			wantBody:   "Laptop",
		},
		{
			name:       "item not found",
			method:     http.MethodGet,
			path:       "/items/999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid ID",
			method:     http.MethodGet,
			path:       "/items/abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodPost,
			path:       "/items/1",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			h.GetItemByID(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body missing %q, got: %s", tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid order",
			method:     http.MethodPost,
			body:       `{"user_id":1,"items":[{"item_id":1,"quantity":1}]}`,
			wantStatus: http.StatusCreated,
			wantBody:   `"status":"paid"`,
		},
		{
			name:       "invalid item",
			method:     http.MethodPost,
			body:       `{"user_id":1,"items":[{"item_id":999,"quantity":1}]}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "not found",
		},
		{
			name:       "empty body",
			method:     http.MethodPost,
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewStore()
			h := NewHandler(s)

			req := httptest.NewRequest(tt.method, "/orders", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.CreateOrder(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body missing %q, got: %s", tt.wantBody, w.Body.String())
			}
		})
	}
}
