package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"firstgo-back/internal/model"
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrMenuItemNotFound = errors.New("menu item not found")
)

// MenuStore persists menu categories and items.
type MenuStore struct {
	db *sql.DB
}

// NewMenuStore creates a menu store.
func NewMenuStore(db *sql.DB) *MenuStore {
	return &MenuStore{db: db}
}

func nowTS() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

// ListMenu returns categories with their items (items sorted by sort_order, name).
func (s *MenuStore) ListMenu(includeUnavailable bool) ([]model.MenuCategoryWithItems, error) {
	cats, err := s.ListCategories()
	if err != nil {
		return nil, err
	}
	out := make([]model.MenuCategoryWithItems, 0, len(cats))
	for _, c := range cats {
		items, err := s.ListItemsByCategory(c.ID, includeUnavailable)
		if err != nil {
			return nil, err
		}
		out = append(out, model.MenuCategoryWithItems{
			MenuCategory: c,
			Items:        items,
		})
	}
	return out, nil
}

// ListCategories lists all categories ordered by sort_order, name.
func (s *MenuStore) ListCategories() ([]model.MenuCategory, error) {
	rows, err := s.db.Query(
		`SELECT id, name, sort_order, created_at FROM menu_categories ORDER BY sort_order ASC, name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.MenuCategory
	for rows.Next() {
		var c model.MenuCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// GetCategoryByID returns one category.
func (s *MenuStore) GetCategoryByID(id string) (model.MenuCategory, error) {
	var c model.MenuCategory
	err := s.db.QueryRow(
		`SELECT id, name, sort_order, created_at FROM menu_categories WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.MenuCategory{}, ErrCategoryNotFound
		}
		return model.MenuCategory{}, err
	}
	return c, nil
}

// CreateCategory inserts a category.
func (s *MenuStore) CreateCategory(name string, sortOrder int) (model.MenuCategory, error) {
	id := uuid.NewString()
	createdAt := nowTS()
	_, err := s.db.Exec(
		`INSERT INTO menu_categories (id, name, sort_order, created_at) VALUES (?, ?, ?, ?)`,
		id, name, sortOrder, createdAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.MenuCategory{}, errors.New("category name already exists")
		}
		return model.MenuCategory{}, err
	}
	return model.MenuCategory{ID: id, Name: name, SortOrder: sortOrder, CreatedAt: createdAt}, nil
}

// UpdateCategory updates name and/or sort_order.
func (s *MenuStore) UpdateCategory(id string, name *string, sortOrder *int) (model.MenuCategory, error) {
	cur, err := s.GetCategoryByID(id)
	if err != nil {
		return model.MenuCategory{}, err
	}
	if name != nil {
		cur.Name = *name
	}
	if sortOrder != nil {
		cur.SortOrder = *sortOrder
	}
	_, err = s.db.Exec(
		`UPDATE menu_categories SET name = ?, sort_order = ? WHERE id = ?`,
		cur.Name, cur.SortOrder, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.MenuCategory{}, errors.New("category name already exists")
		}
		return model.MenuCategory{}, err
	}
	return cur, nil
}

// DeleteCategory removes a category (fails if items still reference it).
func (s *MenuStore) DeleteCategory(id string) error {
	res, err := s.db.Exec(`DELETE FROM menu_categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// ListItems returns all items, optionally only available.
func (s *MenuStore) ListItems(includeUnavailable bool) ([]model.MenuItem, error) {
	q := `SELECT id, category_id, name, description, price, image_url, is_available, sort_order, created_at, updated_at FROM menu_items`
	if !includeUnavailable {
		q += ` WHERE is_available = 1`
	}
	q += ` ORDER BY sort_order ASC, name ASC`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMenuItems(rows)
}

// ListItemsByCategory lists items in one category.
func (s *MenuStore) ListItemsByCategory(categoryID string, includeUnavailable bool) ([]model.MenuItem, error) {
	q := `SELECT id, category_id, name, description, price, image_url, is_available, sort_order, created_at, updated_at FROM menu_items WHERE category_id = ?`
	if !includeUnavailable {
		q += ` AND is_available = 1`
	}
	q += ` ORDER BY sort_order ASC, name ASC`
	rows, err := s.db.Query(q, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMenuItems(rows)
}

// GetItemByID returns one menu item.
func (s *MenuStore) GetItemByID(id string) (model.MenuItem, error) {
	var item model.MenuItem
	var desc, img sql.NullString
	var avail int
	err := s.db.QueryRow(
		`SELECT id, category_id, name, description, price, image_url, is_available, sort_order, created_at, updated_at FROM menu_items WHERE id = ?`,
		id,
	).Scan(&item.ID, &item.CategoryID, &item.Name, &desc, &item.Price, &img, &avail, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.MenuItem{}, ErrMenuItemNotFound
		}
		return model.MenuItem{}, err
	}
	if desc.Valid {
		item.Description = desc.String
	}
	if img.Valid {
		item.ImageURL = img.String
	}
	item.IsAvailable = avail == 1
	return item, nil
}

// CreateItem inserts a menu item.
func (s *MenuStore) CreateItem(req model.CreateMenuItemRequest) (model.MenuItem, error) {
	if _, err := s.GetCategoryByID(req.CategoryID); err != nil {
		return model.MenuItem{}, err
	}
	id := uuid.NewString()
	ts := nowTS()
	avail := 1
	if req.IsAvailable != nil && !*req.IsAvailable {
		avail = 0
	}
	var desc, img interface{}
	if req.Description != "" {
		desc = req.Description
	}
	if req.ImageURL != "" {
		img = req.ImageURL
	}
	_, err := s.db.Exec(
		`INSERT INTO menu_items (id, category_id, name, description, price, image_url, is_available, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.CategoryID, req.Name, desc, req.Price, img, avail, req.SortOrder, ts, ts,
	)
	if err != nil {
		return model.MenuItem{}, err
	}
	return s.GetItemByID(id)
}

// UpdateItem patches a menu item.
func (s *MenuStore) UpdateItem(id string, req model.UpdateMenuItemRequest) (model.MenuItem, error) {
	cur, err := s.GetItemByID(id)
	if err != nil {
		return model.MenuItem{}, err
	}
	if req.CategoryID != nil {
		if _, err := s.GetCategoryByID(*req.CategoryID); err != nil {
			return model.MenuItem{}, err
		}
		cur.CategoryID = *req.CategoryID
	}
	if req.Name != nil {
		cur.Name = *req.Name
	}
	if req.Description != nil {
		cur.Description = *req.Description
	}
	if req.Price != nil {
		cur.Price = *req.Price
	}
	if req.ImageURL != nil {
		cur.ImageURL = *req.ImageURL
	}
	if req.IsAvailable != nil {
		cur.IsAvailable = *req.IsAvailable
	}
	if req.SortOrder != nil {
		cur.SortOrder = *req.SortOrder
	}
	avail := 0
	if cur.IsAvailable {
		avail = 1
	}
	var desc, img interface{}
	if cur.Description != "" {
		desc = cur.Description
	}
	if cur.ImageURL != "" {
		img = cur.ImageURL
	}
	updatedAt := nowTS()
	_, err = s.db.Exec(
		`UPDATE menu_items SET category_id = ?, name = ?, description = ?, price = ?, image_url = ?, is_available = ?, sort_order = ?, updated_at = ? WHERE id = ?`,
		cur.CategoryID, cur.Name, desc, cur.Price, img, avail, cur.SortOrder, updatedAt, id,
	)
	if err != nil {
		return model.MenuItem{}, err
	}
	cur.UpdatedAt = updatedAt
	return cur, nil
}

// DeleteItem removes a menu item.
func (s *MenuStore) DeleteItem(id string) error {
	res, err := s.db.Exec(`DELETE FROM menu_items WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrMenuItemNotFound
	}
	return nil
}

func scanMenuItems(rows *sql.Rows) ([]model.MenuItem, error) {
	var list []model.MenuItem
	for rows.Next() {
		var item model.MenuItem
		var desc, img sql.NullString
		var avail int
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.Name, &desc, &item.Price, &img, &avail, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			item.Description = desc.String
		}
		if img.Valid {
			item.ImageURL = img.String
		}
		item.IsAvailable = avail == 1
		list = append(list, item)
	}
	return list, rows.Err()
}