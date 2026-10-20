package inventory

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/auth"
	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

type increaseStockHTTPRequest struct {
	WarehouseID    int64   `json:"warehouse_id"`
	SKUID          int64   `json:"sku_id"`
	BatchID        *int64  `json:"batch_id"`
	Qty            string  `json:"qty"`
	IdempotencyKey string  `json:"idempotency_key"`
	SourceType     *string `json:"source_type"`
	SourceID       *string `json:"source_id"`
	SourceLineID   *string `json:"source_line_id"`
}

type reserveStockHTTPRequest struct {
	WarehouseID    int64   `json:"warehouse_id"`
	SKUID          int64   `json:"sku_id"`
	BatchID        *int64  `json:"batch_id"`
	Qty            string  `json:"qty"`
	IdempotencyKey string  `json:"idempotency_key"`
	SourceType     *string `json:"source_type"`
	SourceID       *string `json:"source_id"`
	SourceLineID   *string `json:"source_line_id"`
}

type decreaseStockHTTPRequest struct {
	WarehouseID    int64   `json:"warehouse_id"`
	SKUID          int64   `json:"sku_id"`
	BatchID        *int64  `json:"batch_id"`
	ReservationID  *int64  `json:"reservation_id"`
	Qty            string  `json:"qty"`
	IdempotencyKey string  `json:"idempotency_key"`
	SourceType     *string `json:"source_type"`
	SourceID       *string `json:"source_id"`
	SourceLineID   *string `json:"source_line_id"`
}

type releaseStockHTTPRequest struct {
	WarehouseID    int64   `json:"warehouse_id"`
	SKUID          int64   `json:"sku_id"`
	ReservationID  int64   `json:"reservation_id"`
	Qty            *string `json:"qty"`
	IdempotencyKey string  `json:"idempotency_key"`
	SourceType     *string `json:"source_type"`
	SourceID       *string `json:"source_id"`
	SourceLineID   *string `json:"source_line_id"`
}

// IncreaseStock godoc
// @Summary Increase stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Param body body increaseStockHTTPRequest true "Increase stock payload"
// @Success 200 {object} response.Body{data=StockOperationResult}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/increase [post]
func (h *handler) IncreaseStock(c *gin.Context) {
	var req increaseStockHTTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("invalid request body"))
		return
	}

	result, err := h.service.IncreaseStock(c.Request.Context(), IncreaseStockRequest{
		OperationContext: operationContextFromGin(c, req.IdempotencyKey, req.SourceType, req.SourceID, req.SourceLineID),
		WarehouseID:      req.WarehouseID,
		SKUID:            req.SKUID,
		BatchID:          req.BatchID,
		Qty:              req.Qty,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// ReserveStock godoc
// @Summary Reserve stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Param body body reserveStockHTTPRequest true "Reserve stock payload"
// @Success 200 {object} response.Body{data=StockOperationResult}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/reserve [post]
func (h *handler) ReserveStock(c *gin.Context) {
	var req reserveStockHTTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("invalid request body"))
		return
	}

	result, err := h.service.ReserveStock(c.Request.Context(), ReserveStockRequest{
		OperationContext: operationContextFromGin(c, req.IdempotencyKey, req.SourceType, req.SourceID, req.SourceLineID),
		WarehouseID:      req.WarehouseID,
		SKUID:            req.SKUID,
		BatchID:          req.BatchID,
		Qty:              req.Qty,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// DecreaseStock godoc
// @Summary Decrease stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Param body body decreaseStockHTTPRequest true "Decrease stock payload"
// @Success 200 {object} response.Body{data=StockOperationResult}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/decrease [post]
func (h *handler) DecreaseStock(c *gin.Context) {
	var req decreaseStockHTTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("invalid request body"))
		return
	}

	result, err := h.service.DecreaseStock(c.Request.Context(), DecreaseStockRequest{
		OperationContext: operationContextFromGin(c, req.IdempotencyKey, req.SourceType, req.SourceID, req.SourceLineID),
		WarehouseID:      req.WarehouseID,
		SKUID:            req.SKUID,
		BatchID:          req.BatchID,
		ReservationID:    req.ReservationID,
		Qty:              req.Qty,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// ReleaseStock godoc
// @Summary Release reserved stock
// @Tags Inventory
// @Accept json
// @Produce json
// @Param body body releaseStockHTTPRequest true "Release stock payload"
// @Success 200 {object} response.Body{data=StockOperationResult}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/release [post]
func (h *handler) ReleaseStock(c *gin.Context) {
	var req releaseStockHTTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("invalid request body"))
		return
	}

	result, err := h.service.ReleaseStock(c.Request.Context(), ReleaseStockRequest{
		OperationContext: operationContextFromGin(c, req.IdempotencyKey, req.SourceType, req.SourceID, req.SourceLineID),
		WarehouseID:      req.WarehouseID,
		SKUID:            req.SKUID,
		ReservationID:    req.ReservationID,
		Qty:              req.Qty,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// ListMovements godoc
// @Summary List inventory movements
// @Tags Inventory
// @Produce json
// @Param warehouse_id query int false "Warehouse ID"
// @Param sku_id query int false "SKU ID"
// @Param operation_type query string false "Operation type" Enums(increase,reserve,release_reserved,decrease_available,decrease_reserved)
// @Param created_from query string false "Created from (RFC3339)"
// @Param created_to query string false "Created to (RFC3339, exclusive)"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=MovementListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/movements [get]
func (h *handler) ListMovements(c *gin.Context) {
	filter, err := parseListMovementsFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	result, err := h.service.ListMovements(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

func operationContextFromGin(c *gin.Context, idempotencyKey string, sourceType *string, sourceID *string, sourceLineID *string) OperationContext {
	return OperationContext{
		Actor:          actorFromGin(c),
		IdempotencyKey: idempotencyKey,
		SourceType:     sourceType,
		SourceID:       sourceID,
		SourceLineID:   sourceLineID,
	}
}

func actorFromGin(c *gin.Context) *Actor {
	caller, ok := auth.CallerFromGinContext(c)
	if !ok {
		return nil
	}

	switch caller.Type {
	case auth.CallerTypeAdmin:
		if caller.Admin != nil {
			return &Actor{Type: string(caller.Type), ID: caller.Admin.AdminUserID, Label: caller.Admin.Username}
		}
	case auth.CallerTypeAPIApp:
		if caller.APIApp != nil {
			return &Actor{Type: string(caller.Type), ID: caller.APIApp.APIAppID, Label: caller.APIApp.AppID}
		}
	}

	return nil
}

func parseListMovementsFilter(c *gin.Context) (ListMovementsFilter, error) {
	filter := ListMovementsFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if raw := c.Query("warehouse_id"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return ListMovementsFilter{}, apperr.NewValidationError("warehouse_id must be an integer greater than zero")
		}
		filter.WarehouseID = &value
	}
	if raw := c.Query("sku_id"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return ListMovementsFilter{}, apperr.NewValidationError("sku_id must be an integer greater than zero")
		}
		filter.SKUID = &value
	}
	if raw := c.Query("operation_type"); raw != "" {
		filter.OperationType = &raw
	}
	if raw := c.Query("created_from"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return ListMovementsFilter{}, apperr.NewValidationError("created_from must be an RFC3339 timestamp")
		}
		filter.CreatedFrom = &value
	}
	if raw := c.Query("created_to"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return ListMovementsFilter{}, apperr.NewValidationError("created_to must be an RFC3339 timestamp")
		}
		filter.CreatedTo = &value
	}
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return ListMovementsFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(value)
	}
	if raw := c.Query("offset"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return ListMovementsFilter{}, apperr.NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(value)
	}

	return filter, nil
}
