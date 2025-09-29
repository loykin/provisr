package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ContextKey is used for context keys to avoid collisions
type ContextKey string

const (
	// ResultKey is the context key for auth result
	ResultKey ContextKey = "auth_result"
)

// Middleware provides authentication middleware for HTTP handlers
type Middleware struct {
	authService *AuthService
	enabled     bool
}

// GinAuth returns a Gin middleware function for authentication
func (m *Middleware) GinAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled {
			c.Next()
			return
		}

		authResult, err := m.authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_failed",
				"message": "Authentication required",
			})
			c.Abort()
			return
		}

		if !authResult.Success {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_failed",
				"message": "Invalid credentials",
			})
			c.Abort()
			return
		}

		// Store auth result in context
		c.Set(string(ResultKey), authResult)
		c.Next()
	}
}

// GinRequirePermission returns a Gin middleware that requires specific permissions
func (m *Middleware) GinRequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled {
			c.Next()
			return
		}

		authResult, exists := c.Get(string(ResultKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_required",
				"message": "Authentication required",
			})
			c.Abort()
			return
		}

		result, ok := authResult.(*AuthResult)
		if !ok || !result.Success {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_failed",
				"message": "Invalid authentication",
			})
			c.Abort()
			return
		}

		if !m.authService.HasPermission(result.Roles, resource, action) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "permission_denied",
				"message": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// HTTPAuth returns a standard HTTP middleware function for authentication
func (m *Middleware) HTTPAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		authResult, err := m.authenticate(r)
		if err != nil || !authResult.Success {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"authentication_failed","message":"Authentication required"}`))
			return
		}

		// Store auth result in request context
		ctx := context.WithValue(r.Context(), ResultKey, authResult)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HTTPRequirePermission returns a standard HTTP middleware that requires specific permissions
func (m *Middleware) HTTPRequirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.enabled {
				next.ServeHTTP(w, r)
				return
			}

			authResult := r.Context().Value(ResultKey)
			if authResult == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"authentication_required","message":"Authentication required"}`))
				return
			}

			result, ok := authResult.(*AuthResult)
			if !ok || !result.Success {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"authentication_failed","message":"Invalid authentication"}`))
				return
			}

			if !m.authService.HasPermission(result.Roles, resource, action) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"permission_denied","message":"Insufficient permissions"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// authenticate extracts and validates authentication from HTTP request
func (m *Middleware) authenticate(r *http.Request) (*AuthResult, error) {
	// Try Authorization header first (Bearer token)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			req := LoginRequest{
				Method: AuthMethodJWT,
				Token:  parts[1],
			}
			return m.authService.Authenticate(r.Context(), req)
		}
	}

	// Try Basic Authentication
	username, password, ok := r.BasicAuth()
	if ok {
		req := LoginRequest{
			Method:   AuthMethodBasic,
			Username: username,
			Password: password,
		}
		return m.authService.Authenticate(r.Context(), req)
	}

	// Try client credentials from form/query params
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID != "" && clientSecret != "" {
		req := LoginRequest{
			Method:       AuthMethodClientSecret,
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}
		return m.authService.Authenticate(r.Context(), req)
	}

	return &AuthResult{Success: false}, ErrInvalidCredentials
}
