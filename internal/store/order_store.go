package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"firstgo-back/internal/model"
)

var ErrMenuItemUnavailable = errors.New("menu item unavailable")

// OrderStore persists orders and their line items.
type OrderStore struct {
	db *sql.DB
}

// NewOrderStore creates an order store.
func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// Create writes an order and its items atomically. Prices and names are
// snapshotted from menu_items at order time; the total is computed server-side.
// A missing or unavailable menu item aborts the whole order.
func (s *OrderStore) Create(userID string, req model.CreateOrderRequest) (model.Order, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return model.Order{}, err
	}
	defer tx.Rollback() // no-op once committed

	orderID := uuid.NewString()
	createdAt := nowTS()

	items := make([]model.OrderItem, 0, len(req.Items))
	var total float64
	for _, li := range req.Items {
		var name string
		var price float64
		var avail int
		err := tx.QueryRow(
			`SELECT name, price, is_available FROM menu_items WHERE id = ?`,
			li.MenuItemID,
		).Scan(&name, &price, &avail)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.Order{}, ErrMenuItemNotFound
			}
			return model.Order{}, err
		}
		if avail == 0 {
			return model.Order{}, ErrMenuItemUnavailable
		}
		total += price * float64(li.Quantity)
		items = append(items, model.OrderItem{
			ID:         uuid.NewString(),
			MenuItemID: li.MenuItemID,
			Name:       name,
			Price:      price,
			Quantity:   li.Quantity,
		})
	}

	var note interface{}
	if req.Note != "" {
		note = req.Note
	}
	var tableNo interface{}
	if req.TableNo != "" {
		tableNo = req.TableNo
	}
	if _, err := tx.Exec(
		`INSERT INTO orders (id, user_id, table_no, note, total_amount, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		orderID, userID, tableNo, note, total, createdAt,
	); err != nil {
		return model.Order{}, err
	}
	for _, it := range items {
		if _, err := tx.Exec(
			`INSERT INTO order_items (id, order_id, menu_item_id, name, price, quantity) VALUES (?, ?, ?, ?, ?, ?)`,
			it.ID, orderID, it.MenuItemID, it.Name, it.Price, it.Quantity,
		); err != nil {
			return model.Order{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Order{}, err
	}

	return model.Order{
		ID:          orderID,
		UserID:      userID,
		TableNo:     req.TableNo,
		Note:        req.Note,
		TotalAmount: total,
		CreatedAt:   createdAt,
		Items:       items,
	}, nil
}

// List returns all orders with the ordering user's username and their items (admin).
func (s *OrderStore) List() ([]model.Order, error) {
	rows, err := s.db.Query(
		`SELECT o.id, o.user_id, u.username, COALESCE(o.table_no,''), o.note, o.total_amount, o.created_at
		   FROM orders o
		   JOIN users u ON u.id = o.user_id
		  ORDER BY o.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.Order, 0)
	index := make(map[string]int)
	for rows.Next() {
		var o model.Order
		var note sql.NullString
		if err := rows.Scan(&o.ID, &o.UserID, &o.Username, &o.TableNo, &note, &o.TotalAmount, &o.CreatedAt); err != nil {
			return nil, err
		}
		if note.Valid {
			o.Note = note.String
		}
		o.Items = []model.OrderItem{}
		index[o.ID] = len(orders)
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return orders, nil
	}
	return s.fillOrderItems(orders, index)
}

// ListByUser returns orders for a specific user with their items.
func (s *OrderStore) ListByUser(userID string) ([]model.Order, error) {
	rows, err := s.db.Query(
		`SELECT o.id, o.user_id, COALESCE(o.table_no,''), o.note, o.total_amount, o.created_at
		   FROM orders o
		  WHERE o.user_id = ?
		  ORDER BY o.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.Order, 0)
	index := make(map[string]int)
	for rows.Next() {
		var o model.Order
		var note sql.NullString
		if err := rows.Scan(&o.ID, &o.UserID, &o.TableNo, &note, &o.TotalAmount, &o.CreatedAt); err != nil {
			return nil, err
		}
		if note.Valid {
			o.Note = note.String
		}
		o.Items = []model.OrderItem{}
		index[o.ID] = len(orders)
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return orders, nil
	}
	return s.fillOrderItems(orders, index)
}

func (s *OrderStore) fillOrderItems(orders []model.Order, index map[string]int) ([]model.Order, error) {
	itemRows, err := s.db.Query(
		`SELECT id, order_id, menu_item_id, name, price, quantity FROM order_items`,
	)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var it model.OrderItem
		var orderID string
		if err := itemRows.Scan(&it.ID, &orderID, &it.MenuItemID, &it.Name, &it.Price, &it.Quantity); err != nil {
			return nil, err
		}
		if idx, ok := index[orderID]; ok {
			orders[idx].Items = append(orders[idx].Items, it)
		}
	}
	return orders, itemRows.Err()
}
