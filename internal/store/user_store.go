package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"firstgo-back/internal/model"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUserDisabled       = errors.New("user disabled")
)

// UserStore persists users in MySQL.
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
	createdAt := time.Now().UTC().Format("2006-01-02 15:04:05")

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
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// FindOrCreateByOpenID looks up a user by openid, creating one if absent.
// Concurrent inserts are handled via unique-index conflict + retry.
func (s *UserStore) FindOrCreateByOpenID(openid, unionid string) (model.User, error) {
	// Try to find existing user.
	u, err := s.getByOpenID(openid)
	if err == nil {
		// Update last_login_at and unionid if newly provided.
		s.updateLogin(openid, unionid)
		return u, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return model.User{}, err
	}

	// Create new user.
	id := uuid.NewString()
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	displayName := fmt.Sprintf("顾客%.5s", id)

	_, err = s.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, openid, unionid, status, created_at, updated_at, last_login_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
		id, displayName, displayName+"@wechat", []byte{}, openid, unionid, ts, ts, ts,
	)
	if err != nil {
		if isUniqueViolation(err) {
			// Concurrent insert won the race; re-fetch.
			u, err := s.getByOpenID(openid)
			if err != nil {
				return model.User{}, err
			}
			s.updateLogin(openid, unionid)
			return u, nil
		}
		return model.User{}, err
	}

	return model.User{ID: id, Username: displayName, Email: displayName + "@wechat"}, nil
}

// getByOpenID returns a user by openid, respecting status.
func (s *UserStore) getByOpenID(openid string) (model.User, error) {
	var (
		id, uname, email, status string
	)
	err := s.db.QueryRow(
		`SELECT id, username, email, status FROM users WHERE openid = ?`,
		openid,
	).Scan(&id, &uname, &email, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	if status == "disabled" {
		return model.User{}, ErrUserDisabled
	}
	return model.User{ID: id, Username: uname, Email: email}, nil
}

// updateLogin refreshes last_login_at and fills unionid if not yet set.
func (s *UserStore) updateLogin(openid, unionid string) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	if unionid != "" {
		s.db.Exec(
			`UPDATE users SET last_login_at = ?, unionid = CASE WHEN unionid IS NULL THEN ? ELSE unionid END WHERE openid = ?`,
			ts, unionid, openid,
		)
	} else {
		s.db.Exec(`UPDATE users SET last_login_at = ? WHERE openid = ?`, ts, openid)
	}
}