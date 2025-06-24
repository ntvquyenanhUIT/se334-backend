package models

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type User struct {
	ID           int       `db:"id"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (r *RegisterRequest) Validate() error {
	if strings.TrimSpace(r.Username) == "" {
		return errors.New("username cannot be empty")
	}
	if len(r.Username) < 3 || len(r.Username) > 50 {
		return errors.New("username must be between 3 and 50 characters")
	}

	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	if !emailRegex.MatchString(r.Email) {
		return errors.New("invalid email format")
	}

	if len(r.Password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type Token struct {
	ID        int       `db:"id"`
	UserID    int       `db:"user_id"`
	Token     string    `db:"token"`
	CreatedAt time.Time `db:"created_at"`
	IsRevoked bool      `db:"is_revoked"`
	ExpiresAt time.Time `db:"expires_at"`
}
