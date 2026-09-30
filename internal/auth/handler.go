package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
)

type Handler interface {
	RegisterRoutes(router gin.IRouter, adminSessionMiddleware gin.HandlerFunc, passwordChangeSessionMiddleware gin.HandlerFunc)
}

type CookieOptions struct {
	Name     string
	Path     string
	Secure   bool
	SameSite http.SameSite
}

type handler struct {
	service Service
	cookie  CookieOptions
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Admin     AdminIdentity `json:"admin"`
	ExpiresAt time.Time     `json:"expires_at"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type CreateAPIAppRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Status      Status   `json:"status"`
	Metadata    Metadata `json:"metadata"`
}

type UpdateAPIAppRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Status      Status   `json:"status"`
	Metadata    Metadata `json:"metadata"`
}

type IssueAPISecretRequest struct {
	Name          string     `json:"name"`
	BoundMetadata Metadata   `json:"bound_metadata"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

type APISecretListResponse struct {
	Items []APISecret `json:"items"`
}

func NewHandler(service Service, cookie CookieOptions) Handler {
	if cookie.Name == "" {
		cookie.Name = DefaultSessionCookieName
	}
	if cookie.Path == "" {
		cookie.Path = "/api/v1"
	}
	if cookie.SameSite == 0 {
		cookie.SameSite = http.SameSiteLaxMode
	}
	return &handler{service: service, cookie: cookie}
}

func (h *handler) RegisterRoutes(router gin.IRouter, adminSessionMiddleware gin.HandlerFunc, passwordChangeSessionMiddleware gin.HandlerFunc) {
	authRoutes := router.Group("/auth")
	authRoutes.POST("/admin/login", h.Login)

	restricted := authRoutes.Group("")
	restricted.Use(passwordChangeSessionMiddleware)
	restricted.POST("/admin/logout", h.Logout)
	restricted.GET("/admin/me", h.Me)
	restricted.PUT("/admin/password", h.ChangePassword)

	admin := authRoutes.Group("")
	admin.Use(adminSessionMiddleware)
	admin.GET("/apps", h.ListAPIApps)
	admin.POST("/apps", h.CreateAPIApp)
	admin.GET("/apps/:id", h.GetAPIApp)
	admin.PUT("/apps/:id", h.UpdateAPIApp)
	admin.DELETE("/apps/:id", h.DeleteAPIApp)
	admin.POST("/apps/:id/secrets", h.IssueAPISecret)
	admin.GET("/apps/:id/secrets", h.ListAPISecrets)
	admin.POST("/apps/:id/secrets/:secret_id/block", h.BlockAPISecret)
}

// Login logs in an administrator and sets the admin session cookie.
// @Summary Log in as an administrator
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Administrator login payload"
// @Success 200 {object} response.Body{data=LoginResponse}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 429 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/admin/login [post]
func (h *handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	result, err := h.service.Login(c.Request.Context(), LoginInput{Username: req.Username, Password: req.Password, ClientIP: c.ClientIP()})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	response.Success(c, LoginResponse{Admin: result.Admin, ExpiresAt: result.ExpiresAt})
}

// ChangePassword changes the current administrator password and renews the session.
// @Summary Change the administrator password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body ChangePasswordRequest true "Password change payload"
// @Success 200 {object} response.Body{data=LoginResponse}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/admin/password [put]
func (h *handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}
	token, err := c.Cookie(h.cookie.Name)
	if err != nil {
		httperr.Write(c, ErrUnauthorized)
		return
	}
	result, err := h.service.ChangePassword(c.Request.Context(), ChangePasswordInput{Token: token, CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword})
	if err != nil {
		httperr.Write(c, err)
		return
	}
	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	response.Success(c, LoginResponse{Admin: result.Admin, ExpiresAt: result.ExpiresAt})
}

// Logout invalidates the current administrator session.
// @Summary Log out the current administrator
// @Tags Authentication
// @Produce json
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/admin/logout [post]
func (h *handler) Logout(c *gin.Context) {
	token, err := c.Cookie(h.cookie.Name)
	if err != nil {
		httperr.Write(c, ErrUnauthorized)
		return
	}
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		httperr.Write(c, err)
		return
	}

	h.clearSessionCookie(c)
	response.NoContent(c)
}

// Me returns the current administrator identity.
// @Summary Get the current administrator
// @Tags Authentication
// @Produce json
// @Success 200 {object} response.Body{data=AdminIdentity}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/admin/me [get]
func (h *handler) Me(c *gin.Context) {
	token, err := c.Cookie(h.cookie.Name)
	if err != nil {
		httperr.Write(c, ErrUnauthorized)
		return
	}
	admin, err := h.service.Me(c.Request.Context(), token)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, admin)
}

// ListAPIApps lists registered API applications.
// @Summary List API applications
// @Tags API Applications
// @Produce json
// @Param status query string false "API application status" Enums(active,inactive)
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=APIAppListResult}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps [get]
func (h *handler) ListAPIApps(c *gin.Context) {
	filter, err := parseAPIAppListFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	result, err := h.service.ListAPIApps(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, result)
}

// GetAPIApp returns an API application by its internal ID.
// @Summary Get an API application
// @Tags API Applications
// @Produce json
// @Param id path int true "API application ID"
// @Success 200 {object} response.Body{data=APIApp}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps/{id} [get]
func (h *handler) GetAPIApp(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	app, err := h.service.GetAPIApp(c.Request.Context(), id)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, app)
}

// CreateAPIApp registers an API application.
// @Summary Create an API application
// @Tags API Applications
// @Accept json
// @Produce json
// @Param body body CreateAPIAppRequest true "API application payload"
// @Success 201 {object} response.Body{data=APIApp}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps [post]
func (h *handler) CreateAPIApp(c *gin.Context) {
	var req CreateAPIAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}
	app, err := h.service.CreateAPIApp(c.Request.Context(), CreateAPIAppInput{
		Name: req.Name, Description: req.Description, Status: req.Status, Metadata: req.Metadata,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Created(c, app)
}

// UpdateAPIApp updates an API application's basic information.
// @Summary Update an API application
// @Tags API Applications
// @Accept json
// @Produce json
// @Param id path int true "API application ID"
// @Param body body UpdateAPIAppRequest true "API application payload"
// @Success 200 {object} response.Body{data=APIApp}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps/{id} [put]
func (h *handler) UpdateAPIApp(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	var req UpdateAPIAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}
	app, err := h.service.UpdateAPIApp(c.Request.Context(), UpdateAPIAppInput{
		ID: id, Name: req.Name, Description: req.Description, Status: req.Status, Metadata: req.Metadata,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, app)
}

// DeleteAPIApp soft deletes an API application.
// @Summary Delete an API application
// @Tags API Applications
// @Produce json
// @Param id path int true "API application ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps/{id} [delete]
func (h *handler) DeleteAPIApp(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	if err := h.service.DeleteAPIApp(c.Request.Context(), id); err != nil {
		httperr.Write(c, err)
		return
	}
	response.NoContent(c)
}

// IssueAPISecret issues a new API secret whose plaintext value is returned only once.
// @Summary Issue an API application secret
// @Description The plaintext secret is returned only in this response and is not stored by Stock-Flow.
// @Tags API Applications
// @Accept json
// @Produce json
// @Param id path int true "API application ID"
// @Param body body IssueAPISecretRequest true "API secret payload"
// @Success 201 {object} response.Body{data=IssuedSecret}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps/{id}/secrets [post]
func (h *handler) IssueAPISecret(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	var req IssueAPISecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}
	secret, err := h.service.IssueAPISecret(c.Request.Context(), IssueSecretInput{
		APIAppID: id, Name: req.Name, BoundMetadata: req.BoundMetadata, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Created(c, secret)
}

// ListAPISecrets lists API secret records without plaintext secret values.
// @Summary List API application secrets
// @Tags API Applications
// @Produce json
// @Param id path int true "API application ID"
// @Success 200 {object} response.Body{data=APISecretListResponse}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps/{id}/secrets [get]
func (h *handler) ListAPISecrets(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	secrets, err := h.service.ListAPISecrets(c.Request.Context(), id)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, APISecretListResponse{Items: secrets})
}

// BlockAPISecret blocks an API secret from authenticating future requests.
// @Summary Block an API application secret
// @Tags API Applications
// @Produce json
// @Param id path int true "API application ID"
// @Param secret_id path string true "API secret identifier"
// @Success 200 {object} response.Body{data=APISecret}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /auth/apps/{id}/secrets/{secret_id}/block [post]
func (h *handler) BlockAPISecret(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	secret, err := h.service.BlockAPISecret(c.Request.Context(), id, c.Param("secret_id"))
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, secret)
}

func (h *handler) setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    token,
		Path:     h.cookie.Path,
		Expires:  expiresAt,
		MaxAge:   max(1, int(time.Until(expiresAt).Seconds())),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: h.cookie.SameSite,
	})
}

func (h *handler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     h.cookie.Path,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: h.cookie.SameSite,
	})
}

func parseAPIAppID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.NewValidationError("api app id must be greater than zero")
	}
	return id, nil
}

func parseAPIAppListFilter(c *gin.Context) (ListFilter, error) {
	filter := ListFilter{Limit: DefaultListLimit}
	if raw := c.Query("status"); raw != "" {
		status := Status(raw)
		filter.Status = &status
	}
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(value)
	}
	if raw := c.Query("offset"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(value)
	}
	return filter, nil
}
