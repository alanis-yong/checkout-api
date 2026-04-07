//go:build ignore

package order

// ENTITY
type Order struct {
	id     int
	userID int
	items  []LineItem
	total  int
	status OrderStatus
}

func New(id, userID int) *Order {
	return &Order{
		id:     id,
		userID: userID,
		items:  []LineItem{},
		status: StatusPending,
	}
}

func (o *Order) AddItem(itemID, quantity, price int) error {
	if o.status != StatusPending {
		return ErrOrderNotEditable
	}

	li, err := NewLineItem(itemID, quantity, price)
	if err != nil {
		return err
	}

	o.items = append(o.items, li)
	o.total += li.Subtotal()
	return nil
}

func (o *Order) MarkPaid()           { o.status = StatusPaid }
func (o *Order) MarkFailed()         { o.status = StatusFailed }
func (o *Order) ID() int             { return o.id }
func (o *Order) UserID() int         { return o.userID }
func (o *Order) Total() int          { return o.total }
func (o *Order) Status() OrderStatus { return o.status }

func (o *Order) Items() []LineItem {
	cp := make([]LineItem, len(o.items))
	//defensive copy!
	copy(cp, o.items)
	return cp
}
