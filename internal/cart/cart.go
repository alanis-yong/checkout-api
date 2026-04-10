package cart

import (
	"errors"
)

var (
	ErrItemNotInCart   = errors.New("item not found in cart")
	ErrInvalidQuantity = errors.New("quantity must be greater than zero")
)

// Cart is our Aggregate Root.
// Note: 'items' is lowercase (private) so handlers can't bypass our rules!
type Cart struct {
	userID int
	items  []LineItem
}

// LineItem is a Value Object representing an item in the cart.
type LineItem struct {
	itemID   int
	quantity int
	price    int // Price at the time it was added
}

// Getters for LineItem (since fields are private)
func (li LineItem) ItemID() int   { return li.itemID }
func (li LineItem) Quantity() int { return li.quantity }
func (li LineItem) Price() int    { return li.price }
func (li LineItem) Subtotal() int { return li.price * li.quantity }

// New is a Factory function to create a valid Cart
func New(userID int) *Cart {
	return &Cart{
		userID: userID,
		items:  []LineItem{},
	}
}

// UserID getter
func (c *Cart) UserID() int { return c.userID }

// Items returns a "Defensive Copy".
// This prevents external code from changing the slice behind our back.
func (c *Cart) Items() []LineItem {
	cp := make([]LineItem, len(c.items))
	copy(cp, c.items)
	return cp
}

// AddItem handles the "Logic": If item exists, update it. If not, add it.
func (c *Cart) AddItem(itemID, quantity, price int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	for i, item := range c.items {
		if item.itemID == itemID {
			// Rule: We just update the quantity if it already exists
			c.items[i].quantity += quantity
			return nil
		}
	}

	// New item
	c.items = append(c.items, LineItem{
		itemID:   itemID,
		quantity: quantity,
		price:    price,
	})
	return nil
}

// RemoveItem removes a specific item
func (c *Cart) RemoveItem(itemID int) error {
	for i, item := range c.items {
		if item.itemID == itemID {
			c.items = append(c.items[:i], c.items[i+1:]...)
			return nil
		}
	}
	return ErrItemNotInCart
}

// IsEmpty is a helper for the Checkout service
func (c *Cart) IsEmpty() bool {
	return len(c.items) == 0
}
