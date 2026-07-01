package store

import (
	"errors"
	"sync"
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

type storedUser struct {
	ID           string
	Username     string
	Email        string
	PasswordHash []byte
	CreatedAt    time.Time
}

// UserStore is an in-memory user repository (replace with DB later).
type UserStore struct {
	mu       sync.RWMutex
	byID     map[string]*storedUser
	byName   map[string]*storedUser
	byEmail  map[string]*storedUser
}

// NewUserStore creates an empty user store.
func NewUserStore() *UserStore {
	return &UserStore{
		byID:    make(map[string]*storedUser),
		byName:  make(map[string]*storedUser),
		byEmail: make(map[string]*storedUser),
	}
}

// Register creates a new user.
func (s *UserStore) Register(username, email, password string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.byName[username]; ok {
		return model.User{}, ErrUsernameTaken
	}
	if _, ok := s.byEmail[email]; ok {
		return model.User{}, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}

	u := &storedUser{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	s.byID[u.ID] = u
	s.byName[u.Username] = u
	s.byEmail[u.Email] = u

	return model.User{ID: u.ID, Username: u.Username, Email: u.Email}, nil
}

// Login verifies credentials and returns the public user.
func (s *UserStore) Login(username, password string) (model.User, error) {
	s.mu.RLock()
	u, ok := s.byName[username]
	s.mu.RUnlock()
	if !ok {
		return model.User{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)); err != nil {
		return model.User{}, ErrInvalidCredentials
	}
	return model.User{ID: u.ID, Username: u.Username, Email: u.Email}, nil
}

// GetByID returns a user by ID (for /me).
func (s *UserStore) GetByID(id string) (model.User, error) {
	s.mu.RLock()
	u, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok {
		return model.User{}, ErrUserNotFound
	}
	return model.User{ID: u.ID, Username: u.Username, Email: u.Email}, nil
}