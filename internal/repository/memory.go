package repository

import (
	"errors"
	"sync"

	"binary-core/internal/models"
)

var (
	ErrUserNotFound      = errors.New("utilisateur non trouvé")
	ErrUserAlreadyExists = errors.New("un utilisateur avec cet email existe déjà")
)

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*models.User // Email -> User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*models.User),
	}
}

func (r *UserRepository) Create(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.Email]; exists {
		return ErrUserAlreadyExists
	}

	r.users[user.Email] = user
	return nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[email]
	if !exists {
		return nil, ErrUserNotFound
	}

	return user, nil
}
