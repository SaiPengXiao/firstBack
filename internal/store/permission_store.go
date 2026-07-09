package store

import (
	"database/sql"
	"errors"
)

// PermissionStore persists RBAC relations (roles, permissions, user_roles).
type PermissionStore struct {
	db *sql.DB
}

// NewPermissionStore creates a permission store backed by the given database.
func NewPermissionStore(db *sql.DB) *PermissionStore {
	return &PermissionStore{db: db}
}

// HasPermission reports whether the user has the given permission code through
// any of their roles (permissions are the union of all roles a user holds).
func (s *PermissionStore) HasPermission(userID, code string) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1
		   FROM user_roles ur
		   JOIN role_permissions rp ON rp.role_id = ur.role_id
		   JOIN permissions p ON p.id = rp.permission_id
		  WHERE ur.user_id = ? AND p.code = ?
		  LIMIT 1`,
		userID, code,
	).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetRolesByUserID returns the names of all roles assigned to the user.
func (s *PermissionStore) GetRolesByUserID(userID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT r.name
		   FROM user_roles ur
		   JOIN roles r ON r.id = ur.role_id
		  WHERE ur.user_id = ?
		  ORDER BY r.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}
