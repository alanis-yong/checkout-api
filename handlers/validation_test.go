package handlers

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
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
