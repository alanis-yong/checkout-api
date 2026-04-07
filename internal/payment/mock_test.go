package payment

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMock_Charge_Valid(t *testing.T) {
	m := Mock{}
	txnID, err := m.Charge(context.Background(), 1000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.HasPrefix(txnID, "txn_") {
		t.Errorf("expected txnID to start with 'txn_', got %q", txnID)
	}
}

func TestMock_Charge_ZeroAmount(t *testing.T) {
	m := Mock{}
	_, err := m.Charge(context.Background(), 0)
	if !errors.Is(err, ErrChargeFailed) {
		t.Errorf("expected ErrChargeFailed, got %v", err)
	}
}

func TestMock_Charge_NegativeAmount(t *testing.T) {
	m := Mock{}
	_, err := m.Charge(context.Background(), -500)
	if !errors.Is(err, ErrChargeFailed) {
		t.Errorf("expected ErrChargeFailed, got %v", err)
	}
}

func TestMock_Charge_TooLarge(t *testing.T) {
	m := Mock{}
	_, err := m.Charge(context.Background(), 1_000_000)
	if !errors.Is(err, ErrChargeFailed) {
		t.Errorf("expected ErrChargeFailed, got %v", err)
	}
}
