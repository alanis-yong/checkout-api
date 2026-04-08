package order

// Order is an aggregate root — all mutations go through its methods.
// External code cannot access items directly.
type Order struct {
	// TODO: add private fields — id, userID int, items []LineItem, total int, status OrderStatus
	id     int
	userID int
	items  []LineItem
	total  int
	status OrderStatus
}

// New creates a new pending order for the given user.
func New(id, userID int) *Order {
	// TODO: initialize with StatusPending and an empty items slice
	return &Order{
		id:     id,
		userID: userID,
		items:  []LineItem{},
		total:  0,
		status: StatusPending,
	}
}

// AddItem is the only entry point for adding items to an order.
// Returns ErrOrderNotEditable if the order is no longer pending.
func (o *Order) AddItem(itemID, quantity, price int) error {
	// TODO: return ErrOrderNotEditable if o.status != StatusPending
	// TODO: create a LineItem via NewLineItem — propagate any error
	if o.status != StatusPending {
		return ErrOrderNotEditable
	}
	lineItem, err := NewLineItem(itemID, quantity, price)
	if err != nil {
		return err
	}
	// TODO: append to o.items and accumulate o.total
	o.items = append(o.items, lineItem)
	o.total += lineItem.Subtotal()
	return nil
}

// MarkPaid and MarkFailed are the only ways to transition status.
// No one can write o.status = "shiped".
func (o *Order) MarkPaid() {
	o.status = StatusPaid
} // TODO: set o.status = StatusPaid
func (o *Order) MarkFailed() {
	o.status = StatusFailed
} // TODO: set o.status = StatusFailed

// Read-only accessors.
func (o *Order) ID() int             { return o.id }
func (o *Order) UserID() int         { return o.userID }
func (o *Order) Total() int          { return o.total }
func (o *Order) Status() OrderStatus { return o.status }

// Items returns a defensive copy — callers cannot mutate the internal slice.
func (o *Order) Items() []LineItem {
	// TODO: return a copy of o.items, not a direct reference
	// hint: make([]LineItem, len(o.items)) + copy()
	itemsCopy := make([]LineItem, len(o.items))
	copy(itemsCopy, o.items)
	return itemsCopy
}
