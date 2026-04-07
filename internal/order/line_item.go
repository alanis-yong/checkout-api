package order

import "errors"

// LineItem is a value object — price is locked at creation time.
// All fields must be private: the only way to create a LineItem is through NewLineItem.
type LineItem struct {
	// TODO: add private fields — itemID int, quantity int, price int
}

// NewLineItem validates and constructs a LineItem.
// Returns ErrInvalidQuantity if quantity <= 0.
// Returns ErrInvalidPrice if price <= 0.
func NewLineItem(itemID, quantity, price int) (LineItem, error) {
	// TODO: validate quantity > 0, return ErrInvalidQuantity if not
	// TODO: validate price > 0, return ErrInvalidPrice if not
	// TODO: return a valid LineItem
	return LineItem{}, errors.New("not implemented")
}

// TODO: implement read-only accessors
func (li LineItem) ItemID() int   { return 0 }
func (li LineItem) Quantity() int { return 0 }
func (li LineItem) Price() int    { return 0 }
func (li LineItem) Subtotal() int { return 0 }

// WithQuantity returns a new LineItem with updated quantity.
// The original is not mutated — value objects are replaced, not changed in place.
func (li LineItem) WithQuantity(qty int) (LineItem, error) {
	// TODO: implement — hint: reuse NewLineItem
	return LineItem{}, errors.New("not implemented")
}
