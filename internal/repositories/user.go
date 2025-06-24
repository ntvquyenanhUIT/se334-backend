package repositories

import (
	"HAB/internal/models"
	"HAB/internal/utils"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	CreateUser(ctx context.Context, req *models.RegisterRequest) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	StoreRefreshToken(ctx context.Context, userID int, token string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, token string) (*models.Token, error)
	RevokeToken(ctx context.Context, token string) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) CreateUser(ctx context.Context, req *models.RegisterRequest) (*models.User, error) {
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	query := `INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, req.Username, req.Email, hashedPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	user := &models.User{
		ID:       int(id),
		Username: req.Username,
		Email:    req.Email,
	}
	return user, nil
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

func (r *userRepository) StoreRefreshToken(ctx context.Context, userID int, token string, expiresAt time.Time) error {
	query := `INSERT INTO tokens (user_id, token, expires_at) VALUES (?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}
	return nil
}

func (r *userRepository) GetRefreshToken(ctx context.Context, token string) (*models.Token, error) {
	var t models.Token
	query := `SELECT id, user_id, token, is_revoked, expires_at FROM tokens WHERE token = ?`
	err := r.db.GetContext(ctx, &t, query, token)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &t, nil
}

func (r *userRepository) RevokeToken(ctx context.Context, token string) error {
	query := `UPDATE tokens SET is_revoked = TRUE WHERE token = ?`
	_, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	return nil
}
