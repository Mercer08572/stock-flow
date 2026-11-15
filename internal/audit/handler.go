package audit

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
)

type Handler interface {
	RegisterRoutes(router gin.IRouter)
}

type handler struct {
	service Service
}

func NewHandler(service Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	router.GET("/audit/logs", h.List)
	router.GET("/audit/stats", h.Stats)
}

// List returns one page of audit rows ordered by occurred_at DESC, id DESC.
// @Summary List request audit logs
// @Tags Audit
// @Produce json
// @Param from query string false "Start time (RFC3339), defaults to to-24h"
// @Param to query string false "End time (RFC3339), defaults to now"
// @Param auth_method query string false "admin_session, api_app, none, or ambiguous"
// @Param actor_type query string false "admin or api_app"
// @Param actor_id query int false "Admin user id or API application id"
// @Param actor_label query string false "Admin username or API application app_id"
// @Param client_ip query string false "Caller IP address"
// @Param status_code query int false "HTTP status code"
// @Param path query string false "Exact request path"
// @Param trace_id query string false "Exact trace id from a previous response"
// @Param limit query int false "Page size, 1-200 (default 50)"
// @Param cursor query string false "Opaque cursor from a previous response"
// @Success 200 {object} response.Body{data=ListResult}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /audit/logs [get]
// @Security AdminSession
func (h *handler) List(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, result)
}

// Stats returns inventory API throughput and failure-rate trend buckets.
// @Summary Get inventory API throughput statistics
// @Description Aggregates audit rows for the inventory mutation routes into time buckets. Failed counts include 4xx and 5xx responses.
// @Tags Audit
// @Produce json
// @Param from query string true "Start time (RFC3339)"
// @Param to query string true "End time (RFC3339, exclusive)"
// @Param granularity query string false "Bucket granularity" Enums(hour,day)
// @Param timezone query string false "IANA timezone used to align buckets" default(Asia/Shanghai)
// @Param routes query string false "Comma-separated route templates; defaults to the inventory mutation routes"
// @Success 200 {object} response.Body{data=Stats}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /audit/stats [get]
// @Security AdminSession
func (h *handler) Stats(c *gin.Context) {
	filter, err := parseStatsFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	result, err := h.service.Stats(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}
	response.Success(c, result)
}

func parseStatsFilter(c *gin.Context) (StatsFilter, error) {
	from, err := parseTimeQuery(c, "from")
	if err != nil {
		return StatsFilter{}, err
	}
	if from.IsZero() {
		return StatsFilter{}, apperr.NewValidationError("from is required")
	}
	to, err := parseTimeQuery(c, "to")
	if err != nil {
		return StatsFilter{}, err
	}
	if to.IsZero() {
		return StatsFilter{}, apperr.NewValidationError("to is required")
	}

	filter := StatsFilter{
		From:        from,
		To:          to,
		Granularity: strings.TrimSpace(c.Query("granularity")),
		Timezone:    strings.TrimSpace(c.Query("timezone")),
	}

	if raw := strings.TrimSpace(c.Query("routes")); raw != "" {
		parts := strings.Split(raw, ",")
		routes := make([]string, 0, len(parts))
		for _, part := range parts {
			if route := strings.TrimSpace(part); route != "" {
				routes = append(routes, route)
			}
		}
		filter.Routes = routes
	}

	return filter, nil
}

func parseListFilter(c *gin.Context) (ListFilter, error) {
	filter := ListFilter{Limit: defaultListLimit}

	from, err := parseTimeQuery(c, "from")
	if err != nil {
		return ListFilter{}, err
	}
	filter.From = from

	to, err := parseTimeQuery(c, "to")
	if err != nil {
		return ListFilter{}, err
	}
	filter.To = to

	if raw := strings.TrimSpace(c.Query("auth_method")); raw != "" {
		value := AuthMethod(raw)
		filter.AuthMethod = &value
	}
	if raw := strings.TrimSpace(c.Query("actor_type")); raw != "" {
		value := ActorType(raw)
		filter.ActorType = &value
	}
	if raw := strings.TrimSpace(c.Query("actor_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("actor_id must be an integer")
		}
		filter.ActorID = &value
	}
	if raw := strings.TrimSpace(c.Query("actor_label")); raw != "" {
		filter.ActorLabel = raw
	}
	if raw := strings.TrimSpace(c.Query("client_ip")); raw != "" {
		filter.ClientIP = raw
	}
	if raw := strings.TrimSpace(c.Query("status_code")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("status_code must be an integer")
		}
		filter.StatusCode = &value
	}
	if raw := strings.TrimSpace(c.Query("path")); raw != "" {
		filter.Path = raw
	}
	if raw := strings.TrimSpace(c.Query("trace_id")); raw != "" {
		filter.TraceID = raw
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = value
	}
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		filter.Cursor = raw
	}

	return filter, nil
}

func parseTimeQuery(c *gin.Context, key string) (time.Time, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, nil
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, apperr.NewValidationError(key + " must be an RFC3339 timestamp")
	}
	return value, nil
}
