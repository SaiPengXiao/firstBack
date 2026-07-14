package model

// OrderItem is a line in an order. Name and price are snapshotted at order time
// so historical orders are unaffected by later menu edits or deletions.
type OrderItem struct {
	ID         string  `json:"id"`
	MenuItemID string  `json:"menuItemId"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Quantity   int     `json:"quantity"`
}

// Order is an order placed by a user. Username is denormalized for the admin list.
type Order struct {
	ID          string      `json:"id"`
	UserID      string      `json:"userId"`
	Username    string      `json:"username,omitempty"`
	TableNo     string      `json:"tableNo,omitempty"`
	Note        string      `json:"note,omitempty"`
	TotalAmount float64     `json:"totalAmount"`
	CreatedAt   string      `json:"createdAt"`
	Items       []OrderItem `json:"items"`
}

// CreateOrderItem is one line of a create-order request.
type CreateOrderItem struct {
	MenuItemID string `json:"menuItemId" binding:"required"`
	Quantity   int    `json:"quantity" binding:"required,min=1"`
}

// CreateOrderRequest body for POST /api/orders.
type CreateOrderRequest struct {
	Items   []CreateOrderItem `json:"items" binding:"required,min=1,dive"`
	TableNo string            `json:"tableNo"`
	Note    string            `json:"note"`
}
