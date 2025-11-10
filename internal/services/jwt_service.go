package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	ErrInvalidTokenType    = errors.New("invalid token type")
	ErrTokenExpired        = errors.New("token expired")
	ErrInvalidIssuer       = errors.New("invalid issuer")
	ErrInvalidAudience     = errors.New("invalid audience")
	ErrInvalidKeyID        = errors.New("invalid key ID")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
	ErrRefreshTokenInvalid = errors.New("refresh token invalid")
)

type TokenClaims struct {
	UserID         uint   `json:"sub"`
	Username       string `json:"username"`
	SessionVersion uint   `json:"session_version"`
	TokenType      string `json:"type"`
	KeyID          string `json:"kid,omitempty"`
	jwt.RegisteredClaims
}

type JWTService struct {
	config      *config.JWTConfig
	userRepo    UserRepository
	refreshRepo repositories.RefreshTokenRepositoryInterface
}

type verificationKey struct {
	key []byte
}

type UserRepository interface {
	GetByID(id uint) (*database.User, error)
	GetByUsername(username string) (*database.User, error)
	Update(user *database.User) error
}

func NewJWTService(cfg *config.JWTConfig, userRepo UserRepository, refreshRepo repositories.RefreshTokenRepositoryInterface) *JWTService {
	return &JWTService{
		config:      cfg,
		userRepo:    userRepo,
		refreshRepo: refreshRepo,
	}
}

func (j *JWTService) GenerateAccessToken(user *database.User) (string, error) {
	return j.generateToken(user, TokenTypeAccess, time.Duration(j.config.Expiration)*time.Hour)
}

func (j *JWTService) GenerateRefreshToken(user *database.User) (string, error) {
	if user == nil {
		return "", ErrInvalidCredentials
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	tokenString := hex.EncodeToString(tokenBytes)
	tokenHash := j.hashToken(tokenString)

	refreshToken := &database.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(time.Duration(j.config.RefreshExpiration) * time.Hour),
	}

	if err := j.refreshRepo.Create(refreshToken); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}

	return tokenString, nil
}

func (j *JWTService) VerifyAccessToken(tokenString string) (uint, error) {
	claims, err := j.parseToken(tokenString)
	if err != nil {
		return 0, err
	}

	if claims.TokenType != TokenTypeAccess {
		return 0, ErrInvalidTokenType
	}

	user, err := j.userRepo.GetByID(claims.UserID)
	if err != nil {
		if IsRecordNotFound(err) {
			return 0, ErrInvalidToken
		}
		return 0, fmt.Errorf("lookup user: %w", err)
	}

	if user.Locked {
		return 0, ErrAccountLocked
	}

	if user.SessionVersion != claims.SessionVersion {
		return 0, ErrInvalidToken
	}

	return claims.UserID, nil
}

func (j *JWTService) RefreshAccessToken(refreshTokenString string) (string, error) {

	tokenHash := j.hashToken(refreshTokenString)

	refreshToken, err := j.refreshRepo.GetByTokenHash(tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrRefreshTokenInvalid
		}
		return "", fmt.Errorf("lookup refresh token: %w", err)
	}

	if time.Now().After(refreshToken.ExpiresAt) {

		j.refreshRepo.DeleteByID(refreshToken.ID)
		return "", ErrRefreshTokenExpired
	}

	user, err := j.userRepo.GetByID(refreshToken.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {

			j.refreshRepo.DeleteByID(refreshToken.ID)
			return "", ErrRefreshTokenInvalid
		}
		return "", fmt.Errorf("lookup user: %w", err)
	}

	if user.Locked {

		j.refreshRepo.DeleteByID(refreshToken.ID)
		return "", ErrAccountLocked
	}

	accessToken, err := j.GenerateAccessToken(user)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	return accessToken, nil
}

func (j *JWTService) RevokeRefreshToken(refreshTokenString string) error {
	tokenHash := j.hashToken(refreshTokenString)
	refreshToken, err := j.refreshRepo.GetByTokenHash(tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("lookup refresh token: %w", err)
	}

	return j.refreshRepo.DeleteByID(refreshToken.ID)
}

func (j *JWTService) RevokeAllRefreshTokens(userID uint) error {
	return j.refreshRepo.DeleteByUserID(userID)
}

func (j *JWTService) CleanupExpiredTokens() error {
	return j.refreshRepo.DeleteExpired()
}

func (j *JWTService) generateToken(user *database.User, tokenType string, expiration time.Duration) (string, error) {
	if user == nil {
		return "", ErrInvalidCredentials
	}

	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", fmt.Errorf("generate token ID: %w", err)
	}

	now := time.Now()
	claims := TokenClaims{
		UserID:         user.ID,
		Username:       user.Username,
		SessionVersion: user.SessionVersion,
		TokenType:      tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(jtiBytes),
			Issuer:    j.config.Issuer,
			Audience:  []string{j.config.Audience},
			Subject:   fmt.Sprint(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiration)),
		},
	}

	if j.config.KeyRotation.Enabled {
		claims.KeyID = j.config.KeyRotation.KeyID
	}

	var signingMethod jwt.SigningMethod
	var key any

	if j.config.KeyRotation.Enabled {
		signingMethod = jwt.SigningMethodHS256
		key = []byte(j.config.KeyRotation.CurrentKey)
	} else {
		signingMethod = jwt.SigningMethodHS256
		key = []byte(j.config.Secret)
	}

	token := jwt.NewWithClaims(signingMethod, claims)

	if j.config.KeyRotation.Enabled {
		token.Header["kid"] = j.config.KeyRotation.KeyID
	}

	return token.SignedString(key)
}

func (j *JWTService) parseToken(tokenString string) (*TokenClaims, error) {
	if TrimString(tokenString) == "" {
		return nil, ErrInvalidToken
	}

	parser := jwt.NewParser()
	unverified, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, ErrInvalidToken
	}
	headerKid, _ := unverified.Header["kid"].(string)

	if j.config.KeyRotation.Enabled {
		if headerKid != "" && headerKid != j.config.KeyRotation.KeyID && j.config.KeyRotation.PreviousKey == "" {
			return nil, ErrInvalidKeyID
		}
	} else if headerKid != "" {
		return nil, ErrInvalidKeyID
	}

	keys := j.verificationKeys()
	if len(keys) == 0 {
		return nil, ErrInvalidToken
	}

	var lastErr error
	for _, candidate := range keys {
		claims := &TokenClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return candidate.key, nil
		})
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return nil, ErrTokenExpired
			}
			lastErr = ErrInvalidToken
			continue
		}

		if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) < 0 {
			return nil, ErrTokenExpired
		}

		if !token.Valid {
			lastErr = ErrInvalidToken
			continue
		}

		if err := j.validateTokenMetadata(token, claims, headerKid); err != nil {
			return nil, err
		}

		return claims, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, ErrInvalidToken
}

func (j *JWTService) verificationKeys() []verificationKey {
	if j.config == nil {
		return nil
	}

	if j.config.KeyRotation.Enabled {
		keys := []verificationKey{{key: []byte(j.config.KeyRotation.CurrentKey)}}
		if j.config.KeyRotation.PreviousKey != "" {
			keys = append(keys, verificationKey{key: []byte(j.config.KeyRotation.PreviousKey)})
		}
		return keys
	}

	return []verificationKey{{key: []byte(j.config.Secret)}}
}

func (j *JWTService) validateTokenMetadata(token *jwt.Token, claims *TokenClaims, headerKid string) error {
	actualKid, _ := token.Header["kid"].(string)
	if actualKid == "" {
		actualKid = headerKid
	}

	if j.config.KeyRotation.Enabled {
		if actualKid == "" {
			if claims.KeyID != "" {
				return ErrInvalidKeyID
			}
		} else {
			if claims.KeyID == "" || claims.KeyID != actualKid {
				return ErrInvalidKeyID
			}
		}

		if actualKid != "" && actualKid != j.config.KeyRotation.KeyID && j.config.KeyRotation.PreviousKey == "" {
			return ErrInvalidKeyID
		}
	} else {
		if actualKid != "" || claims.KeyID != "" {
			return ErrInvalidKeyID
		}
	}

	if claims.Issuer != j.config.Issuer {
		return ErrInvalidIssuer
	}

	if !slices.Contains(claims.Audience, j.config.Audience) {
		return ErrInvalidAudience
	}

	return nil
}

func (j *JWTService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
