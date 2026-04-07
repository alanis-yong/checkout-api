package order

import (
	"errors"
	"testing"
)

func TestOrder_AddItem(t *testing.T) {
	o := New(0, 1)
	if err := o.AddItem(1, 2, 500); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if o.Total() != 1000 {
		t.Errorf("Total = %d, want 1000", o.Total())
	}
	if len(o.Items()) != 1 {
		t.Errorf("Items count = %d, want 1", len(o.Items()))
	}
}

func TestOrder_AddItem_AccumulatesTotal(t *testing.T) {
	o := New(0, 1)
	o.AddItem(1, 2, 500) // 1000
	o.AddItem(2, 1, 300) // 300
	if o.Total() != 1300 {
		t.Errorf("Total = %d, want 1300", o.Total())
	}
}

func TestOrder_AddItem_NotPending(t *testing.T) {
	o := New(0, 1)
	o.AddItem(1, 1, 500)
	o.MarkPaid()

	err := o.AddItem(2, 1, 300)
	if !errors.Is(err, ErrOrderNotEditable) {
		t.Errorf("expected ErrOrderNotEditable, got %v", err)
	}
}

func TestOrder_AddItem_InvalidQuantity(t *testing.T) {
	o := New(0, 1)
	err := o.AddItem(1, 0, 500)
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestOrder_AddItem_InvalidPrice(t *testing.T) {
	o := New(0, 1)
	err := o.AddItem(1, 1, 0)
	if !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("expected ErrInvalidPrice, got %v", err)
	}
}

func TestOrder_MarkPaid(t *testing.T) {
	o := New(0, 1)
	o.MarkPaid()
	if o.Status() != StatusPaid {
		t.Errorf("Status = %v, want StatusPaid", o.Status())
	}
}

func TestOrder_MarkFailed(t *testing.T) {
	o := New(0, 1)
	o.MarkFailed()
	if o.Status() != StatusFailed {
		t.Errorf("Status = %v, want StatusFailed", o.Status())
	}
}

func TestOrder_New_IsPending(t *testing.T) {
	o := New(0, 1)
	if o.Status() != StatusPending {
		t.Errorf("Status = %v, want StatusPending", o.Status())
	}
}

func TestOrder_Items_DefensiveCopy(t *testing.T) {
	o := New(0, 1)
	o.AddItem(1, 2, 500)

	items := o.Items()
	li, _ := NewLineItem(99, 1, 100)
	items[0] = li // mutate the returned slice

	// original must be untouched
	if o.Items()[0].ItemID() != 1 {
		t.Errorf("internal items were mutated — defensive copy is broken")
	}
}
