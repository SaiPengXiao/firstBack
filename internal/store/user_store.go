package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"firstgo-back/internal/model"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrEmailTaken         = errors.New("email already taken")
)

// UserStore persists users in SQLite.
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a user store backed by the given database.
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// Register creates a new user.
func (s *UserStore) Register(username, email, password string) (model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}

	id := uuid.NewString()
	createdAt := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, username, email, hash, createdAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			var exists int
			_ = s.db.QueryRow(`SELECT 1 FROM users WHERE username = ?`, username).Scan(&exists)
			if exists == 1 {
				return model.User{}, ErrUsernameTaken
			}
			return model.User{}, ErrEmailTaken
		}
		return model.User{}, err
	}

	return model.User{ID: id, Username: username, Email: email}, nil
}

// Login verifies credentials and returns the public user.
func (s *UserStore) Login(username, password string) (model.User, error) {
	var id, uname, email string
	var hash []byte
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash FROM users WHERE username = ?`,
		username,
	).Scan(&id, &uname, &email, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrInvalidCredentials
		}
		return model.User{}, err
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return model.User{}, ErrInvalidCredentials
	}

	return model.User{ID: id, Username: uname, Email: email}, nil
}

// GetByID returns a user by ID (for /me).
func (s *UserStore) GetByID(id string) (model.User, error) {
	var uname, email string
	err := s.db.QueryRow(
		`SELECT username, email FROM users WHERE id = ?`,
		id,
	).Scan(&uname, &email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	return model.User{ID: id, Username: uname, Email: email}, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}