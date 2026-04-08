package handlers

import (
	"checkout-api/internal/order"
	"errors"
	"log/slog"
	"net/http"
)

type APIError struct {
	Code    string       `json:"error"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
	TraceID string       `json:"trace_id,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIError{
		Code:    code,
		Message: message,
	})
}

func ErrorHandler(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, order.ErrEmptyCart):
		writeError(w, http.StatusUnprocessableEntity, "INSUFFICIENT_STOCK", "Insufficient stock available")

	case errors.Is(err, order.ErrEmptyCart):
		writeError(w, http.StatusUnprocessableEntity, "EMPTY_CART", "Cannot proceed order with an empty cart")
	case errors.Is(err, order.ErrInsufficientStock):
		writeError(w, http.StatusUnprocessableEntity, "INSUFFICIENT_STOCK", "Insufficient stock method is invalid")
	default:
		slog.Error("undefined error", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong")

	}
}
