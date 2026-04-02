package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/repository"
)

var validUsername = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// bcryptCost is the cost factor for bcrypt password hashing.
// Higher values are more secure but slower. 12 is the recommended minimum.
const bcryptCost = 12

var (
	ErrUsernameTaken   = errors.New("username already taken")
	ErrWeakPassword    = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong = errors.New("password must be at most 72 bytes")
	ErrInvalidUsername = errors.New("username must be 1-32 alphanumeric characters, underscores, or hyphens")
	ErrBadCredentials  = errors.New("invalid username or password")
)

// UserRepo defines what the auth service needs for user persistence.
type UserRepo interface {
	Create(ctx context.Context, provider, providerID, username, email, passwordHash string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	UpsertByProvider(ctx context.Context, provider, providerID, username, avatarURL string) (*model.User, error)
}

type AuthService struct {
	users     UserRepo
	jwtSecret []byte
}

func NewAuthService(users UserRepo, jwtSecret string) (*AuthService, error) {
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes, got %d", len(jwtSecret))
	}
	return &AuthService{
		users:     users,
		jwtSecret: []byte(jwtSecret),
	}, nil
}

func (s *AuthService) Register(ctx context.Context, username, password string) (*model.AuthResponse, error) {
	if username == "" || len(username) > 32 || !validUsername.MatchString(username) {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUsername, ErrValidation)
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("%w: %w", ErrWeakPassword, ErrValidation)
	}
	if len(password) > 72 {
		return nil, fmt.Errorf("%w: %w", ErrPasswordTooLong, ErrValidation)
	}

	existing, err := s.users.GetByUsername(ctx, username)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: %w", ErrUsernameTaken, ErrValidation)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, "local", username, username, "", string(hash))
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := s.generateToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &model.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*model.AuthResponse, error) {
	if username == "" || password == "" {
		return nil, ErrBadCredentials
	}

	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrBadCredentials
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Provider != "local" {
		return nil, ErrBadCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrBadCredentials
	}

	token, err := s.generateToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &model.AuthResponse{Token: token, User: *user}, nil
}

// ValidateToken parses a JWT and returns the user ID.
func (s *AuthService) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token claims")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", fmt.Errorf("missing user id in token")
	}

	return userID, nil
}

func (s *AuthService) generateToken(userID, username string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"exp":      now.Add(24 * time.Hour).Unix(),
		"iat":      now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
