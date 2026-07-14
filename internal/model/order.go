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

// AdminOrderQuery holds filter/pagination params for GET /api/admin/orders.
type AdminOrderQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	Username     string `form:"username"`
	MenuItemName string `form:"menuItemName"`
	StartTime    string `form:"startTime"`
	EndTime      string `form:"endTime"`
}

// AdminOrderItem extends OrderItem with the parent order ID (for joined queries).
type AdminOrderItem struct {
	OrderItem
	OrderID string `json:"-"`
}

// PaginatedOrders is returned by admin order list with pagination info.
type PaginatedOrders struct {
	Orders     []Order          `json:"orders"`
	Pagination Pagination       `json:"pagination"`
}

// Pagination holds page metadata.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// CreateOrderRequest body for POST /api/orders.
type CreateOrderRequest struct {
	Items   []CreateOrderItem `json:"items" binding:"required,min=1,dive"`
	TableNo string            `json:"tableNo"`
	Note    string            `json:"note"`
}
