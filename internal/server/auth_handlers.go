package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/internal/auth"
)

// Common error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Pagination parameters
type PaginationParams struct {
	Offset int
	Limit  int
}

// parsePaginationParams parses offset and limit query parameters
func parsePaginationParams(c *gin.Context) (*PaginationParams, error) {
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "10")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return nil, errors.New("offset must be a number")
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return nil, errors.New("limit must be a number")
	}

	if limit > 100 {
		limit = 100 // Cap at 100
	}

	return &PaginationParams{Offset: offset, Limit: limit}, nil
}

// respondError sends a standardized error response
func respondError(c *gin.Context, statusCode int, errorCode, message string) {
	c.JSON(statusCode, ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}

// handleBindingError handles JSON binding errors
func handleBindingError(c *gin.Context, _ error) {
	respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request format")
}

// handleAuthServiceError handles common auth service errors
func handleAuthServiceError(c *gin.Context, err error, notFoundError error, notFoundCode, failedCode string) {
	if errors.Is(err, notFoundError) {
		respondError(c, http.StatusNotFound, notFoundCode, getErrorMessage(notFoundError))
	} else {
		respondError(c, http.StatusInternalServerError, failedCode, err.Error())
	}
}

func getErrorMessage(err error) string {
	switch err {
	case auth.ErrUserNotFound:
		return "User not found"
	case auth.ErrClientNotFound:
		return "Client not found"
	default:
		return err.Error()
	}
}

// AuthAPI provides authentication-related HTTP endpoints
type AuthAPI struct {
	authService *auth.AuthService
}

// NewAuthAPI creates a new auth API handler
func NewAuthAPI(authService *auth.AuthService) *AuthAPI {
	return &AuthAPI{
		authService: authService,
	}
}

// RegisterAuthEndpoints registers authentication endpoints to the router
func (api *AuthAPI) RegisterAuthEndpoints(r *gin.RouterGroup) {
	group := r.Group("/auth")
	{
		group.POST("/login", api.login)
		group.POST("/users", api.createUser)
		group.GET("/users", api.listUsers)
		group.GET("/users/:id", api.getUser)
		group.PUT("/users/:id", api.updateUser)
		group.DELETE("/users/:id", api.deleteUser)
		group.PUT("/users/:id/password", api.updateUserPassword)

		group.POST("/clients", api.createClient)
		group.GET("/clients", api.listClients)
		group.GET("/clients/:id", api.getClient)
		group.PUT("/clients/:id", api.updateClient)
		group.DELETE("/clients/:id", api.deleteClient)
	}
}

// login handles authentication requests
func (api *AuthAPI) login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleBindingError(c, err)
		return
	}

	result, err := api.authService.Authenticate(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "authentication_failed", err.Error())
		return
	}

	if !result.Success {
		respondError(c, http.StatusUnauthorized, "authentication_failed", "Invalid credentials")
		return
	}

	c.JSON(http.StatusOK, result)
}

// createUser creates a new user
func (api *AuthAPI) createUser(c *gin.Context) {
	var req struct {
		Username string            `json:"username" binding:"required"`
		Password string            `json:"password" binding:"required"`
		Email    string            `json:"email"`
		Roles    []string          `json:"roles"`
		Metadata map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		handleBindingError(c, err)
		return
	}

	user, err := api.authService.CreateUser(
		c.Request.Context(),
		req.Username,
		req.Password,
		req.Email,
		req.Roles,
		req.Metadata,
	)
	if err != nil {
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			respondError(c, http.StatusConflict, "user_exists", "User already exists")
		} else {
			respondError(c, http.StatusInternalServerError, "creation_failed", err.Error())
		}
		return
	}

	c.JSON(http.StatusCreated, user)
}

// listUsers lists all users with pagination
func (api *AuthAPI) listUsers(c *gin.Context) {
	pagination, err := parsePaginationParams(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}

	users, total, err := api.authService.ListUsers(c.Request.Context(), pagination.Offset, pagination.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"total":  total,
		"offset": pagination.Offset,
		"limit":  pagination.Limit,
	})
}

// getUser gets a specific user
func (api *AuthAPI) getUser(c *gin.Context) {
	id := c.Param("id")

	user, err := api.authService.GetUser(c.Request.Context(), id)
	if err != nil {
		handleAuthServiceError(c, err, auth.ErrUserNotFound, "user_not_found", "get_failed")
		return
	}

	c.JSON(http.StatusOK, user)
}

// updateUser updates a user
func (api *AuthAPI) updateUser(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Username string            `json:"username"`
		Email    string            `json:"email"`
		Roles    []string          `json:"roles"`
		Metadata map[string]string `json:"metadata"`
		Active   *bool             `json:"active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		handleBindingError(c, err)
		return
	}

	user, err := api.authService.GetUser(c.Request.Context(), id)
	if err != nil {
		handleAuthServiceError(c, err, auth.ErrUserNotFound, "user_not_found", "get_failed")
		return
	}

	// Update fields
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Roles != nil {
		user.Roles = req.Roles
	}
	if req.Metadata != nil {
		user.Metadata = req.Metadata
	}
	if req.Active != nil {
		user.Active = *req.Active
	}

	if err := api.authService.UpdateUser(c.Request.Context(), user); err != nil {
		respondError(c, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	// Don't return password hash
	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

// deleteUser deletes a user
func (api *AuthAPI) deleteUser(c *gin.Context) {
	id := c.Param("id")

	if err := api.authService.DeleteUser(c.Request.Context(), id); err != nil {
		handleAuthServiceError(c, err, auth.ErrUserNotFound, "user_not_found", "delete_failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}

// updateUserPassword updates a user's password
func (api *AuthAPI) updateUserPassword(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		handleBindingError(c, err)
		return
	}

	if err := api.authService.UpdateUserPassword(c.Request.Context(), id, req.Password); err != nil {
		handleAuthServiceError(c, err, auth.ErrUserNotFound, "user_not_found", "update_failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password updated successfully",
	})
}

// createClient creates a new client credential
func (api *AuthAPI) createClient(c *gin.Context) {
	var req struct {
		Name     string            `json:"name" binding:"required"`
		Scopes   []string          `json:"scopes"`
		Metadata map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		handleBindingError(c, err)
		return
	}

	client, err := api.authService.CreateClient(
		c.Request.Context(),
		req.Name,
		req.Scopes,
		req.Metadata,
	)
	if err != nil {
		if errors.Is(err, auth.ErrClientAlreadyExists) {
			respondError(c, http.StatusConflict, "client_exists", "Client already exists")
		} else {
			respondError(c, http.StatusInternalServerError, "creation_failed", err.Error())
		}
		return
	}

	c.JSON(http.StatusCreated, client)
}

// listClients lists all clients with pagination
func (api *AuthAPI) listClients(c *gin.Context) {
	pagination, err := parsePaginationParams(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}

	clients, total, err := api.authService.ListClients(c.Request.Context(), pagination.Offset, pagination.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}

	// Don't return client secrets in list
	for _, client := range clients {
		client.ClientSecret = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"total":   total,
		"offset":  pagination.Offset,
		"limit":   pagination.Limit,
	})
}

// getClient gets a specific client
func (api *AuthAPI) getClient(c *gin.Context) {
	id := c.Param("id")

	client, err := api.authService.GetClient(c.Request.Context(), id)
	if err != nil {
		handleAuthServiceError(c, err, auth.ErrClientNotFound, "client_not_found", "get_failed")
		return
	}

	// Don't return client secret
	client.ClientSecret = ""
	c.JSON(http.StatusOK, client)
}

// updateClient updates a client
func (api *AuthAPI) updateClient(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name     string            `json:"name"`
		Scopes   []string          `json:"scopes"`
		Metadata map[string]string `json:"metadata"`
		Active   *bool             `json:"active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		handleBindingError(c, err)
		return
	}

	client, err := api.authService.GetClient(c.Request.Context(), id)
	if err != nil {
		handleAuthServiceError(c, err, auth.ErrClientNotFound, "client_not_found", "get_failed")
		return
	}

	// Update fields
	if req.Name != "" {
		client.Name = req.Name
	}
	if req.Scopes != nil {
		client.Scopes = req.Scopes
	}
	if req.Metadata != nil {
		client.Metadata = req.Metadata
	}
	if req.Active != nil {
		client.Active = *req.Active
	}

	if err := api.authService.UpdateClient(c.Request.Context(), client); err != nil {
		respondError(c, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	// Don't return client secret
	client.ClientSecret = ""
	c.JSON(http.StatusOK, client)
}

// deleteClient deletes a client
func (api *AuthAPI) deleteClient(c *gin.Context) {
	id := c.Param("id")

	if err := api.authService.DeleteClient(c.Request.Context(), id); err != nil {
		handleAuthServiceError(c, err, auth.ErrClientNotFound, "client_not_found", "delete_failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Client deleted successfully",
	})
}
