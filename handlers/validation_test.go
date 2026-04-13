package handlers

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"

	"checkout-api/internal/cart"
	"errors"
	"net/http"
)

func TestCreateCartRequest_Boundaries(t *testing.T) {
	v := validator.New()

	tests := []struct {
		name    string
		req     CreateCartRequest
		wantErr bool
	}{
		{
			name: "Valid Case",
			req: CreateCartRequest{
				UserID: 1,
				Items: []struct {
					ItemID   int `json:"item_id" validate:"required,gt=0"`
					Quantity int `json:"quantity" validate:"required,gt=0"`
				}{{ItemID: 1, Quantity: 1}},
			},
			wantErr: false,
		},
		{
			name: "Boundary: UserID is 0",
			req: CreateCartRequest{
				UserID: 0,
				Items: []struct {
					ItemID   int `json:"item_id" validate:"required,gt=0"`
					Quantity int `json:"quantity" validate:"required,gt=0"`
				}{{ItemID: 1, Quantity: 1}},
			},
			wantErr: true,
		},
		{
			name: "Boundary: ItemID is 0",
			req: CreateCartRequest{
				UserID: 1,
				Items: []struct {
					ItemID   int `json:"item_id" validate:"required,gt=0"`
					Quantity int `json:"quantity" validate:"required,gt=0"`
				}{
					{ItemID: 0, Quantity: 1},
				},
			},
			wantErr: true,
		},
		{
			name: "Boundary: Quantity is 0",
			req: CreateCartRequest{
				UserID: 1,
				Items: []struct {
					ItemID   int `json:"item_id" validate:"required,gt=0"`
					Quantity int `json:"quantity" validate:"required,gt=0"`
				}{
					{ItemID: 1, Quantity: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "Boundary: Items list is empty",
			req: CreateCartRequest{
				UserID: 1,
				Items: []struct {
					ItemID   int `json:"item_id" validate:"required,gt=0"`
					Quantity int `json:"quantity" validate:"required,gt=0"`
				}{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Struct(tt.req)
			if tt.wantErr {
				assert.Error(t, err, "Expected error for boundary: %s", tt.name)
			} else {
				assert.NoError(t, err, "Expected no error for: %s", tt.name)
			}
		})
	}
}

func TestHandler_MapErrorToStatus(t *testing.T) {
	h := &Handler{} // No dependencies needed for this utility test

	tests := []struct {
		name         string
		inputErr     error
		expectedCode int
	}{
		{
			name:         "404: Cart Not Found",
			inputErr:     cart.ErrCartNotFound,
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "402: Payment Failed",
			inputErr:     cart.ErrPaymentFailed,
			expectedCode: http.StatusPaymentRequired,
		},
		{
			name:         "400: Invalid Quantity",
			inputErr:     cart.ErrInvalidQuantity,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "500: Unknown Error",
			inputErr:     errors.New("something went wrong in the db"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.mapErrorToStatus(tt.inputErr)
			assert.Equal(t, tt.expectedCode, got, "Mapping %v should result in status %d", tt.inputErr, tt.expectedCode)
		})
	}
}
