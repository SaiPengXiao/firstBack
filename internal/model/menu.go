package model

// MenuCategory is a menu section (e.g. 热菜).
type MenuCategory struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// MenuItem is a dish/drink on the menu.
type MenuItem struct {
	ID          string  `json:"id"`
	CategoryID  string  `json:"categoryId"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"imageUrl,omitempty"`
	IsAvailable bool    `json:"isAvailable"`
	SortOrder   int     `json:"sortOrder"`
	CreatedAt   string  `json:"createdAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt,omitempty"`
}

// MenuCategoryWithItems is used for GET /api/menu.
type MenuCategoryWithItems struct {
	MenuCategory
	Items []MenuItem `json:"items"`
}

// CreateCategoryRequest body for POST /api/menu/categories.
type CreateCategoryRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=64"`
	SortOrder int    `json:"sortOrder"`
}

// UpdateCategoryRequest body for PUT /api/menu/categories/:id.
type UpdateCategoryRequest struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sortOrder"`
}

// CreateMenuItemRequest body for POST /api/menu/items.
type CreateMenuItemRequest struct {
	CategoryID  string  `json:"categoryId" binding:"required"`
	Name        string  `json:"name" binding:"required,min=1,max=128"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"gte=0"`
	ImageURL    string  `json:"imageUrl"`
	IsAvailable *bool   `json:"isAvailable"`
	SortOrder   int     `json:"sortOrder"`
}

// UpdateMenuItemRequest body for PUT /api/menu/items/:id.
type UpdateMenuItemRequest struct {
	CategoryID  *string  `json:"categoryId"`
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	ImageURL    *string  `json:"imageUrl"`
	IsAvailable *bool    `json:"isAvailable"`
	SortOrder   *int     `json:"sortOrder"`
}