package handlers

type UpsertCartItemRequest struct {
	Quantity int `json:"quantity" validate:"required,gt=0"`
}

type CreateOrderRequest struct {
	LineItems []LineItemRequest `json:"line_items"`
	Total     int               `json:"total"`
}

type LineItemRequest struct {
	ItemID   int `json:"item_id"`
	Quantity int `json:"quantity"`
	Price    int `json:"price"`
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateCartRequest struct {
	UserID int `json:"user_id" validate:"required,gt=0"`
	Items  []struct {
		ItemID   int `json:"item_id" validate:"required,gt=0"`
		Quantity int `json:"quantity" validate:"required,gt=0"`
	} `json:"items" validate:"required,min=1,dive"`
}
