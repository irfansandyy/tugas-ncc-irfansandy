package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"app-backend/models"
	"app-backend/repositories"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailInUse         = errors.New("email already in use")
)

type AuthService struct {
	userRepo  repositories.UserRepository
	jwtSecret []byte
	tokenTTL  time.Duration
}

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func NewAuthService(userRepo repositories.UserRepository, jwtSecret string, tokenTTL time.Duration) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  tokenTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, username string) (models.User, error) {
	if _, err := s.userRepo.GetByEmail(ctx, email); err == nil {
		return models.User{}, ErrEmailInUse
	} else if !errors.Is(err, repositories.ErrUserNotFound) {
		return models.User{}, err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		parts := strings.SplitN(email, "@", 2)
		username = strings.TrimSpace(parts[0])
	}
	if username == "" {
		username = "user"
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, err
	}

	return s.userRepo.CreateUser(ctx, email, string(hashBytes), username)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return "", models.User{}, ErrInvalidCredentials
		}
		return "", models.User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", models.User{}, ErrInvalidCredentials
	}

	token, err := s.createJWT(user.ID)
	if err != nil {
		return "", models.User{}, err
	}

	return token, user, nil
}

func (s *AuthService) ParseToken(tokenString string) (int64, error) {
	parsedToken, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return 0, ErrInvalidCredentials
	}

	return claims.UserID, nil
}

func (s *AuthService) createJWT(userID int64) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   "auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) GetProfile(ctx context.Context, userID int64) (models.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) UpdateUsername(ctx context.Context, userID int64, username string) (models.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 2 || len(username) > 32 {
		return models.User{}, errors.New("username must be between 2 and 32 characters")
	}

	return s.userRepo.UpdateUsername(ctx, userID, username)
}
