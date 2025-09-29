package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
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

// NewAuthServiceWithStore creates a new authentication service with defaults
func NewAuthServiceWithStore(store Store) *AuthService {
	return &AuthService{
		store:      store,
		jwtSecret:  []byte("default-secret-change-in-production"),
		tokenTTL:   24 * time.Hour,
		bcryptCost: bcrypt.DefaultCost,
	}
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
			return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
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
	case AuthMethodClientSecret:
		return s.authenticateClientSecret(ctx, req.ClientID, req.ClientSecret)
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

// authenticateClientSecret performs client_id/client_secret authentication
func (s *AuthService) authenticateClientSecret(ctx context.Context, clientID, clientSecret string) (*AuthResult, error) {
	if clientID == "" || clientSecret == "" {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	client, err := s.store.GetClientByClientID(ctx, clientID)
	if err != nil {
		if err == ErrClientNotFound {
			return &AuthResult{Success: false}, ErrInvalidCredentials
		}
		return &AuthResult{Success: false}, fmt.Errorf("failed to get client: %w", err)
	}

	if !client.Active {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	if subtle.ConstantTimeCompare([]byte(client.ClientSecret), []byte(clientSecret)) != 1 {
		return &AuthResult{Success: false}, ErrInvalidCredentials
	}

	// For client credentials, we create a token with client info
	token, err := s.generateClientJWT(client)
	if err != nil {
		return &AuthResult{Success: false}, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResult{
		Success:  true,
		UserID:   client.ID,
		Username: client.ClientID,
		Roles:    client.Scopes, // Use scopes as roles for clients
		Metadata: client.Metadata,
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

// generateClientJWT generates a JWT token for a client credential
func (s *AuthService) generateClientJWT(client *ClientCredential) (*Token, error) {
	expiresAt := time.Now().Add(s.tokenTTL)

	claims := &Claims{
		UserID:   client.ID,
		Username: client.ClientID,
		Roles:    client.Scopes,
		Metadata: client.Metadata,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "provisr",
			Subject:   client.ID,
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

// CreateClient creates a new client credential
func (s *AuthService) CreateClient(ctx context.Context, name string, scopes []string, metadata map[string]string) (*ClientCredential, error) {
	if name == "" {
		return nil, fmt.Errorf("client name is required")
	}

	clientID := generateClientID()
	clientSecret := generateClientSecret()

	client := &ClientCredential{
		ID:           generateID(),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Name:         name,
		Scopes:       scopes,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Active:       true,
	}

	if err := s.store.CreateClient(ctx, client); err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
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

// HasPermission checks if a user has a specific permission
func (s *AuthService) HasPermission(userRoles []string, resource, action string) bool {
	// Define role permissions
	rolePermissions := map[string][]Permission{
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

	for _, role := range userRoles {
		permissions, exists := rolePermissions[role]
		if !exists {
			continue
		}

		for _, perm := range permissions {
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
	return s.store.UpdateUser(ctx, user)
}

// DeleteUser deletes a user
func (s *AuthService) DeleteUser(ctx context.Context, id string) error {
	return s.store.DeleteUser(ctx, id)
}

// ListUsers lists users with pagination
func (s *AuthService) ListUsers(ctx context.Context, offset, limit int) ([]*User, int, error) {
	return s.store.ListUsers(ctx, offset, limit)
}

// GetClient gets a client by ID
func (s *AuthService) GetClient(ctx context.Context, id string) (*ClientCredential, error) {
	return s.store.GetClient(ctx, id)
}

// UpdateClient updates a client
func (s *AuthService) UpdateClient(ctx context.Context, client *ClientCredential) error {
	return s.store.UpdateClient(ctx, client)
}

// DeleteClient deletes a client
func (s *AuthService) DeleteClient(ctx context.Context, id string) error {
	return s.store.DeleteClient(ctx, id)
}

// ListClients lists clients with pagination
func (s *AuthService) ListClients(ctx context.Context, offset, limit int) ([]*ClientCredential, int, error) {
	return s.store.ListClients(ctx, offset, limit)
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

func generateClientID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return "client_" + hex.EncodeToString(bytes)[:16]
}

func generateClientSecret() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
