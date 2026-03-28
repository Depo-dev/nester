package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	WalletAddress *string    `json:"walletAddress,omitempty"`
	Name          string     `json:"name"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByWalletAddress(ctx context.Context, addr string) (*User, error)
}
