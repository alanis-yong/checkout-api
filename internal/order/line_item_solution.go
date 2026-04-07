//go:build ignore

package order

// VALUE OBJECT
type LineItem struct {
	itemID   int
	quantity int
	price    int
}

func NewLineItem(itemID, quantity, price int) (LineItem, error) {
	if quantity <= 0 {
		return LineItem{}, ErrInvalidQuantity
	}

	if price <= 0 {
		return LineItem{}, ErrInvalidPrice
	}
	return LineItem{itemID: itemID, quantity: quantity, price: price}, nil
}

func (li LineItem) ItemID() int   { return li.itemID }
func (li LineItem) Quantity() int { return li.quantity }
func (li LineItem) Price() int    { return li.price }
func (li LineItem) Subtotal() int { return li.quantity * li.price }

func (li LineItem) WithQuantity(qty int) (LineItem, error) {
	return NewLineItem(li.itemID, qty, li.price)
}
