package repositories

import (
	"HAB/internal/models"
	"HAB/internal/services"
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
	GetRefreshToken(ctx context.Context, token string) (int, error)
	RevokeToken(ctx context.Context, token string) error
	GetUserInfo(ctx context.Context, userID int) (*models.UserInfo, error)
}

type userRepository struct {
	db    *sqlx.DB
	cache services.Cache
}

func NewUserRepository(db *sqlx.DB, cache services.Cache) UserRepository {
	return &userRepository{db: db, cache: cache}
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
	key := fmt.Sprintf("refresh_token:%s", token)
	ttl := time.Until(expiresAt)

	if ttl <= 0 {
		return fmt.Errorf("token expiration is in the past")
	}

	err := r.cache.Set(ctx, key, userID, ttl)
	if err != nil {
		return fmt.Errorf("failed to store refresh token in cache: %w", err)
	}
	return nil
}

func (r *userRepository) GetRefreshToken(ctx context.Context, token string) (int, error) {
	key := fmt.Sprintf("refresh_token:%s", token)
	var userID int

	err := r.cache.Get(ctx, key, &userID)
	if err != nil {
		return 0, fmt.Errorf("refresh token not found in cache: %w", err)
	}
	return userID, nil
}

func (r *userRepository) RevokeToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	err := r.cache.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to revoke token from cache: %w", err)
	}
	return nil
}

func (r *userRepository) GetUserInfo(ctx context.Context, userID int) (*models.UserInfo, error) {
	var userInfo models.UserInfo
	userQuery := `SELECT username, email, created_at FROM users WHERE id = ?`
	err := r.db.GetContext(ctx, &userInfo, userQuery, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %d", userID)
		}
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	submissionsQuery := `
        SELECT id, language_id, status, submitted_at 
        FROM submissions 
        WHERE user_id = ? 
        ORDER BY submitted_at DESC`
	err = r.db.SelectContext(ctx, &userInfo.Submissions, submissionsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user submissions: %w", err)
	}

	return &userInfo, nil
}
