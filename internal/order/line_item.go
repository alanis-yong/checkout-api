package order

// LineItem is a value object — price is locked at creation time.
// All fields must be private: the only way to create a LineItem is through NewLineItem.
type LineItem struct {
	// TODO: add private fields — itemID int, quantity int, price int
	itemID   int
	quantity int
	price    int
}

// NewLineItem validates and constructs a LineItem.
// Returns ErrInvalidQuantity if quantity <= 0.
// Returns ErrInvalidPrice if price <= 0.
func NewLineItem(itemID, quantity, price int) (LineItem, error) {
	if quantity <= 0 {
		return LineItem{}, ErrInvalidQuantity
	}
	if price <= 0 {
		return LineItem{}, ErrInvalidPrice
	}
	return LineItem{itemID: itemID, quantity: quantity, price: price}, nil
}

// TODO: implement read-only accessors
func (li LineItem) ItemID() int   { return li.itemID }
func (li LineItem) Quantity() int { return li.quantity }
func (li LineItem) Price() int    { return li.price }
func (li LineItem) Subtotal() int { return li.quantity * li.price }

// WithQuantity returns a new LineItem with updated quantity.
// The original is not mutated — value objects are replaced, not changed in place.
func (li LineItem) WithQuantity(qty int) (LineItem, error) {
	// TODO: implement — hint: reuse NewLineItem
	return NewLineItem(li.ItemID(), qty, li.Price())
}
