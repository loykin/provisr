package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthService provides authentication functionality
type AuthService struct {
	store      Store
	jwtSecret  []byte
	tokenTTL   time.Duration
	bcryptCost int
	userMu     sync.Mutex
}

// AuthConfig represents configuration for the auth service
type AuthConfig struct {
	Store      StoreConfig   `toml:"store" yaml:"store" json:"store"`
	JWTSecret  string        `toml:"jwt_secret" yaml:"jwt_secret" json:"jwt_secret"`
	TokenTTL   time.Duration `toml:"token_ttl" yaml:"token_ttl" json:"token_ttl"`
	BcryptCost int           `toml:"bcrypt_cost" yaml:"bcrypt_cost" json:"bcrypt_cost"`
}

// Claims represents JWT claims
type Claims struct {
	UserID   string            `json:"user_id"`
	Username string            `json:"username"`
	Roles    []string          `json:"roles"`
	Metadata map[string]string `json:"metadata"`
	jwt.RegisteredClaims
}

// NewAuthServiceWithStore creates a new authentication service with defaults,
// generating a random JWT secret for this instance.
func NewAuthServiceWithStore(store Store) (*AuthService, error) {
	jwtSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
	}

	return &AuthService{
		store:      store,
		jwtSecret:  jwtSecret,
		tokenTTL:   24 * time.Hour,
		bcryptCost: bcrypt.DefaultCost,
	}, nil
}

// NewAuthService creates a new authentication service
func NewAuthService(config AuthConfig) (*AuthService, error) {
	store, err := NewStore(config.Store)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	jwtSecret := []byte(config.JWTSecret)
	if len(jwtSecret) == 0 {
		// Generate a random secret if not provided
		jwtSecret = make([]byte, 32)
		if _, err := rand.Read(jwtSecret); err != nil {
			return nil, errors.Join(
				fmt.Errorf("failed to generate JWT secret: %w", err),
				store.Close(),
			)
		}
	}

	tokenTTL := config.TokenTTL
	if tokenTTL == 0 {
		tokenTTL = 24 * time.Hour // Default to 24 hours
	}

	bcryptCost := config.BcryptCost
	if bcryptCost == 0 {
		bcryptCost = bcrypt.DefaultCost
	}

	return &AuthService{
		store:      store,
		jwtSecret:  jwtSecret,
		tokenTTL:   tokenTTL,
		bcryptCost: bcryptCost,
	}, nil
}

// Authenticate performs authentication based on the login request
func (s *AuthService) Authenticate(ctx context.Context, req LoginRequest) (*AuthResult, error) {
	switch req.Method {
	case AuthMethodBasic:
		return s.authenticateBasic(ctx, req.Username, req.Password)
	case AuthMethodJWT:
		return s.authenticateJWT(ctx, req.Token)
	default:
		return &AuthResult{Success: false}, fmt.Errorf("unsupported auth method: %s", req.Method)
	}
}

// authenticateBasic performs username/password authentication
func (s *AuthService) authenticateBasic(ctx context.Context, username, password string) (*AuthResult, error) {
	if username == "" || password == "" {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		if err == ErrUserNotFound {
			return &AuthResult{Success: false}, ErrInvalidCredentials
		}
		return &AuthResult{Success: false}, fmt.Errorf("failed to get user: %w", err)
	}

	if !user.Active {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return &AuthResult{Success: false}, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResult{
		Success:  true,
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		Metadata: user.Metadata,
		Token:    token,
	}, nil
}

// authenticateJWT validates a JWT token
func (s *AuthService) authenticateJWT(_ context.Context, tokenString string) (*AuthResult, error) {
	if tokenString == "" {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	return &AuthResult{
		Success:  true,
		UserID:   claims.UserID,
		Username: claims.Username,
		Roles:    claims.Roles,
		Metadata: claims.Metadata,
	}, nil
}

// generateJWT generates a JWT token for a user
func (s *AuthService) generateJWT(user *User) (*Token, error) {
	expiresAt := time.Now().Add(s.tokenTTL)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		Metadata: user.Metadata,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "provisr",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &Token{
		Type:      "Bearer",
		Value:     tokenString,
		ExpiresAt: expiresAt,
	}, nil
}

// CreateUser creates a new user with hashed password
func (s *AuthService) CreateUser(ctx context.Context, username, password, email string, roles []string, metadata map[string]string) (*User, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		ID:           generateID(),
		Username:     username,
		PasswordHash: string(passwordHash),
		Email:        email,
		Roles:        roles,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Active:       true,
	}

	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Don't return password hash
	user.PasswordHash = ""
	return user, nil
}

// HasAnyUsers reports whether the store has at least one user, used to
// decide whether the frontend should show a first-run "create the admin
// account" form instead of a login form.
func (s *AuthService) HasAnyUsers(ctx context.Context) (bool, error) {
	users, _, err := s.store.ListUsers(ctx, 0, 1)
	if err != nil {
		return false, fmt.Errorf("failed to check for existing users: %w", err)
	}
	return len(users) > 0, nil
}

// BootstrapFirstAdmin creates the first admin user directly from an
// unauthenticated request, and returns the same *AuthResult shape as
// Authenticate (including a token) so the caller ends up logged in
// immediately. Refuses with ErrAlreadyBootstrapped once any user exists —
// this is the only auth mutation allowed with no token, so it must stay
// unusable after the very first admin is created.
func (s *AuthService) BootstrapFirstAdmin(ctx context.Context, username, password string) (*AuthResult, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	now := time.Now().UTC()
	user := &User{ID: generateID(), Username: username, PasswordHash: string(passwordHash), Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now, Active: true}
	if err := s.store.CreateFirstUser(ctx, user); err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			return nil, ErrAlreadyBootstrapped
		}
		return nil, fmt.Errorf("failed to create initial admin: %w", err)
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResult{
		Success:  true,
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		Token:    token,
	}, nil
}

// UpdateUserPassword updates a user's password
func (s *AuthService) UpdateUserPassword(ctx context.Context, userID, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("password cannot be empty")
	}

	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = string(passwordHash)
	user.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// rolePermissions defines the permissions for each role.
var rolePermissions = map[string][]Permission{
	"admin": {
		{Resource: "*", Action: "*"}, // Admin has all permissions
	},
	"operator": {
		{Resource: "process", Action: "read"},
		{Resource: "process", Action: "write"},
		{Resource: "job", Action: "read"},
		{Resource: "job", Action: "write"},
		{Resource: "cronjob", Action: "read"},
		{Resource: "cronjob", Action: "write"},
	},
	"viewer": {
		{Resource: "process", Action: "read"},
		{Resource: "job", Action: "read"},
		{Resource: "cronjob", Action: "read"},
	},
}

// HasPermission checks if a user has a specific permission
func (s *AuthService) HasPermission(userRoles []string, resource, action string) bool {
	for _, role := range userRoles {
		for _, perm := range rolePermissions[role] {
			if (perm.Resource == "*" || perm.Resource == resource) &&
				(perm.Action == "*" || perm.Action == action) {
				return true
			}
		}
	}
	return false
}

// GetUser gets a user by ID
func (s *AuthService) GetUser(ctx context.Context, id string) (*User, error) {
	return s.store.GetUser(ctx, id)
}

// UpdateUser updates a user
func (s *AuthService) UpdateUser(ctx context.Context, user *User) error {
	return s.UpdateUserWithPassword(ctx, user, "")
}

// UpdateUserWithPassword updates profile fields and, when supplied, the
// password hash in the same store write. This prevents the UI's combined
// "Save" action from partially applying profile changes before a separate
// password request fails.
func (s *AuthService) UpdateUserWithPassword(ctx context.Context, user *User, newPassword string) error {
	s.userMu.Lock()
	defer s.userMu.Unlock()
	current, err := s.store.GetUser(ctx, user.ID)
	if err != nil {
		return err
	}
	if isActiveAdmin(current) && !isActiveAdmin(user) {
		last, err := s.isLastActiveAdmin(ctx, current.ID)
		if err != nil {
			return err
		}
		if last {
			return ErrLastActiveAdmin
		}
	}
	user.PasswordHash = current.PasswordHash
	if newPassword != "" {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = string(passwordHash)
	}
	return s.store.UpdateUser(ctx, user)
}

// DeleteUser deletes a user
func (s *AuthService) DeleteUser(ctx context.Context, id string) error {
	s.userMu.Lock()
	defer s.userMu.Unlock()
	current, err := s.store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if isActiveAdmin(current) {
		last, err := s.isLastActiveAdmin(ctx, id)
		if err != nil {
			return err
		}
		if last {
			return ErrLastActiveAdmin
		}
	}
	return s.store.DeleteUser(ctx, id)
}

func isActiveAdmin(user *User) bool {
	if user == nil || !user.Active {
		return false
	}
	for _, role := range user.Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}

func (s *AuthService) isLastActiveAdmin(ctx context.Context, exceptID string) (bool, error) {
	users, _, err := s.store.ListUsers(ctx, 0, int(^uint(0)>>1))
	if err != nil {
		return false, fmt.Errorf("failed to list users: %w", err)
	}
	for _, candidate := range users {
		if candidate.ID != exceptID && isActiveAdmin(candidate) {
			return false, nil
		}
	}
	return true, nil
}

// ListUsers lists users with pagination
func (s *AuthService) ListUsers(ctx context.Context, offset, limit int) ([]*User, int, error) {
	return s.store.ListUsers(ctx, offset, limit)
}

// Store returns the underlying store (for CLI operations)
func (s *AuthService) Store() Store {
	return s.store
}

// Close closes the auth service
func (s *AuthService) Close() error {
	return s.store.Close()
}

// Helper functions

func generateID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
