package auth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
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

func (h *handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAuthError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	result, err := h.service.Login(c.Request.Context(), LoginInput{Username: req.Username, Password: req.Password, ClientIP: c.ClientIP()})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	response.Success(c, LoginResponse{Admin: result.Admin, ExpiresAt: result.ExpiresAt})
}

func (h *handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAuthError(c, NewValidationError("request body must be valid JSON"))
		return
	}
	token, err := c.Cookie(h.cookie.Name)
	if err != nil {
		writeAuthError(c, ErrUnauthorized)
		return
	}
	result, err := h.service.ChangePassword(c.Request.Context(), ChangePasswordInput{Token: token, CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword})
	if err != nil {
		writeAuthError(c, err)
		return
	}
	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	response.Success(c, LoginResponse{Admin: result.Admin, ExpiresAt: result.ExpiresAt})
}

func (h *handler) Logout(c *gin.Context) {
	token, err := c.Cookie(h.cookie.Name)
	if err != nil {
		writeAuthError(c, ErrUnauthorized)
		return
	}
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		writeAuthError(c, err)
		return
	}

	h.clearSessionCookie(c)
	response.NoContent(c)
}

func (h *handler) Me(c *gin.Context) {
	token, err := c.Cookie(h.cookie.Name)
	if err != nil {
		writeAuthError(c, ErrUnauthorized)
		return
	}
	admin, err := h.service.Me(c.Request.Context(), token)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Success(c, admin)
}

func (h *handler) ListAPIApps(c *gin.Context) {
	filter, err := parseAPIAppListFilter(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	result, err := h.service.ListAPIApps(c.Request.Context(), filter)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *handler) GetAPIApp(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	app, err := h.service.GetAPIApp(c.Request.Context(), id)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Success(c, app)
}

func (h *handler) CreateAPIApp(c *gin.Context) {
	var req CreateAPIAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAuthError(c, NewValidationError("request body must be valid JSON"))
		return
	}
	app, err := h.service.CreateAPIApp(c.Request.Context(), CreateAPIAppInput{
		Name: req.Name, Description: req.Description, Status: req.Status, Metadata: req.Metadata,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Created(c, app)
}

func (h *handler) UpdateAPIApp(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	var req UpdateAPIAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAuthError(c, NewValidationError("request body must be valid JSON"))
		return
	}
	app, err := h.service.UpdateAPIApp(c.Request.Context(), UpdateAPIAppInput{
		ID: id, Name: req.Name, Description: req.Description, Status: req.Status, Metadata: req.Metadata,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Success(c, app)
}

func (h *handler) DeleteAPIApp(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	if err := h.service.DeleteAPIApp(c.Request.Context(), id); err != nil {
		writeAuthError(c, err)
		return
	}
	response.NoContent(c)
}

func (h *handler) IssueAPISecret(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	var req IssueAPISecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAuthError(c, NewValidationError("request body must be valid JSON"))
		return
	}
	secret, err := h.service.IssueAPISecret(c.Request.Context(), IssueSecretInput{
		APIAppID: id, Name: req.Name, BoundMetadata: req.BoundMetadata, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Created(c, secret)
}

func (h *handler) ListAPISecrets(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	secrets, err := h.service.ListAPISecrets(c.Request.Context(), id)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.Success(c, gin.H{"items": secrets})
}

func (h *handler) BlockAPISecret(c *gin.Context) {
	id, err := parseAPIAppID(c)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	secret, err := h.service.BlockAPISecret(c.Request.Context(), id, c.Param("secret_id"))
	if err != nil {
		writeAuthError(c, err)
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
		return 0, NewValidationError("api app id must be greater than zero")
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
			return ListFilter{}, NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(value)
	}
	if raw := c.Query("offset"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return ListFilter{}, NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(value)
	}
	return filter, nil
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case IsValidationError(err), errors.Is(err, ErrAmbiguousAuth):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrUnauthorized), errors.Is(err, ErrSessionExpired):
		response.Error(c, http.StatusUnauthorized, CodeUnauthorized, "authentication failed")
	case errors.Is(err, ErrPasswordChangeRequired):
		response.Error(c, http.StatusForbidden, CodeForbidden, err.Error())
	case errors.Is(err, ErrRateLimited):
		var limited *RateLimitError
		if errors.As(err, &limited) {
			seconds := int64(limited.RetryAfter.Round(time.Second) / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.FormatInt(seconds, 10))
		}
		response.Error(c, http.StatusTooManyRequests, response.CodeBadRequest, "too many login attempts")
	case errors.Is(err, ErrAPIAppNotFound), errors.Is(err, ErrAPISecretNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrDuplicateAppID), errors.Is(err, ErrDuplicateSecretID):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
