package order

import (
	"errors"
	"testing"
)

func TestNewLineItem_Valid(t *testing.T) {
	li, err := NewLineItem(1, 2, 500)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if li.ItemID() != 1 {
		t.Errorf("ItemID = %d, want 1", li.ItemID())
	}
	if li.Quantity() != 2 {
		t.Errorf("Quantity = %d, want 2", li.Quantity())
	}
	if li.Price() != 500 {
		t.Errorf("Price = %d, want 500", li.Price())
	}
}

func TestNewLineItem_InvalidQuantity(t *testing.T) {
	_, err := NewLineItem(1, 0, 500)
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}

	_, err = NewLineItem(1, -1, 500)
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Errorf("expected ErrInvalidQuantity for negative quantity, got %v", err)
	}
}

func TestNewLineItem_InvalidPrice(t *testing.T) {
	_, err := NewLineItem(1, 2, 0)
	if !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("expected ErrInvalidPrice, got %v", err)
	}

	_, err = NewLineItem(1, 2, -100)
	if !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("expected ErrInvalidPrice for negative price, got %v", err)
	}
}

func TestLineItem_Subtotal(t *testing.T) {
	li, _ := NewLineItem(1, 3, 200)
	if li.Subtotal() != 600 {
		t.Errorf("Subtotal = %d, want 600", li.Subtotal())
	}
}

func TestLineItem_WithQuantity(t *testing.T) {
	li, _ := NewLineItem(1, 2, 500)
	updated, err := li.WithQuantity(5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Quantity() != 5 {
		t.Errorf("Quantity = %d, want 5", updated.Quantity())
	}
	// original must not be mutated
	if li.Quantity() != 2 {
		t.Errorf("original Quantity changed to %d, want 2", li.Quantity())
	}
}

func TestLineItem_WithQuantity_Invalid(t *testing.T) {
	li, _ := NewLineItem(1, 2, 500)
	_, err := li.WithQuantity(0)
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}
