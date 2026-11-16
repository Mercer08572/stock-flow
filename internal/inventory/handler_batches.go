package inventory

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
)

// BatchHandler 负责批次主数据的 HTTP 入口。
type BatchHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type batchHandler struct {
	service BatchService
}

// NewBatchHandler 通过构造函数注入批次服务。
func NewBatchHandler(service BatchService) BatchHandler {
	return &batchHandler{service: service}
}

// CreateBatchRequest 是新增批次的请求体。
//
// production_date / expiration_date 使用 `YYYY-MM-DD`；dispatch_priority 省略时按 10 处理。
type CreateBatchRequest struct {
	SKUID            int64   `json:"sku_id"`
	BatchNo          string  `json:"batch_no"`
	ProductionDate   *string `json:"production_date"`
	ExpirationDate   *string `json:"expiration_date"`
	Status           *string `json:"status"`
	DispatchPriority *int32  `json:"dispatch_priority"`
	Remark           *string `json:"remark"`
}

// UpdateBatchRequest 是编辑批次的请求体。
//
// sku_id 与 batch_no 不可修改；status / dispatch_priority 省略时保留原值。
type UpdateBatchRequest struct {
	ProductionDate   *string `json:"production_date"`
	ExpirationDate   *string `json:"expiration_date"`
	Status           *string `json:"status"`
	DispatchPriority *int32  `json:"dispatch_priority"`
	Remark           *string `json:"remark"`
}

func (h *batchHandler) RegisterRoutes(router gin.IRouter) {
	batches := router.Group("/inventory/batches")
	batches.GET("", h.List)
	batches.GET("/:id", h.Get)
	batches.POST("", h.Create)
	batches.PUT("/:id", h.Update)
	batches.PUT("/:id/disable", h.Disable)
	batches.DELETE("/:id", h.Delete)
}

// List godoc
// @Summary List inventory batches
// @Tags Inventory
// @Produce json
// @Param sku_id query int false "SKU ID"
// @Param status query string false "Batch status" Enums(active,inactive)
// @Param q query string false "Keyword matched against batch number (case-insensitive substring)"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=BatchListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/batches [get]
func (h *batchHandler) List(c *gin.Context) {
	filter, err := parseListBatchesFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	result, err := h.service.ListBatches(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// Get godoc
// @Summary Get an inventory batch
// @Tags Inventory
// @Produce json
// @Param id path int true "Batch ID"
// @Success 200 {object} response.Body{data=BatchDetail}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/batches/{id} [get]
func (h *batchHandler) Get(c *gin.Context) {
	id, err := parseBatchID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	batch, err := h.service.GetBatch(c.Request.Context(), id)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, batch)
}

// Create godoc
// @Summary Create an inventory batch
// @Tags Inventory
// @Accept json
// @Produce json
// @Param body body CreateBatchRequest true "Batch payload"
// @Success 201 {object} response.Body{data=Batch}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/batches [post]
func (h *batchHandler) Create(c *gin.Context) {
	var req CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	input := CreateBatchInput{
		SKUID:            req.SKUID,
		BatchNo:          req.BatchNo,
		ProductionDate:   req.ProductionDate,
		ExpirationDate:   req.ExpirationDate,
		DispatchPriority: req.DispatchPriority,
		Remark:           req.Remark,
	}
	if req.Status != nil {
		input.Status = BatchStatus(*req.Status)
	}

	batch, err := h.service.CreateBatch(c.Request.Context(), input)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Created(c, batch)
}

// Update godoc
// @Summary Update an inventory batch
// @Tags Inventory
// @Accept json
// @Produce json
// @Param id path int true "Batch ID"
// @Param body body UpdateBatchRequest true "Batch payload"
// @Success 200 {object} response.Body{data=Batch}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/batches/{id} [put]
func (h *batchHandler) Update(c *gin.Context) {
	id, err := parseBatchID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	var req UpdateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	input := UpdateBatchInput{
		ID:               id,
		ProductionDate:   req.ProductionDate,
		ExpirationDate:   req.ExpirationDate,
		DispatchPriority: req.DispatchPriority,
		Remark:           req.Remark,
	}
	if req.Status != nil {
		status := BatchStatus(*req.Status)
		input.Status = &status
	}

	batch, err := h.service.UpdateBatch(c.Request.Context(), input)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, batch)
}

// Disable godoc
// @Summary Disable an inventory batch
// @Tags Inventory
// @Produce json
// @Param id path int true "Batch ID"
// @Success 200 {object} response.Body{data=Batch}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/batches/{id}/disable [put]
func (h *batchHandler) Disable(c *gin.Context) {
	id, err := parseBatchID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	batch, err := h.service.DisableBatch(c.Request.Context(), id)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, batch)
}

// Delete godoc
// @Summary Delete an inventory batch
// @Description 仅当批次没有任何库存层或库存流水引用时才允许删除；否则请改用停用。
// @Tags Inventory
// @Produce json
// @Param id path int true "Batch ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/batches/{id} [delete]
func (h *batchHandler) Delete(c *gin.Context) {
	id, err := parseBatchID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	if err := h.service.DeleteBatch(c.Request.Context(), id); err != nil {
		httperr.Write(c, err)
		return
	}

	response.NoContent(c)
}

func parseBatchID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.NewValidationError("batch id must be greater than zero")
	}
	return id, nil
}

func parseListBatchesFilter(c *gin.Context) (ListBatchesFilter, error) {
	filter := ListBatchesFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if rawSKUID := c.Query("sku_id"); rawSKUID != "" {
		skuID, err := strconv.ParseInt(rawSKUID, 10, 64)
		if err != nil {
			return ListBatchesFilter{}, apperr.NewValidationError("sku_id must be an integer")
		}
		filter.SKUID = &skuID
	}

	if rawStatus := c.Query("status"); rawStatus != "" {
		status := BatchStatus(rawStatus)
		filter.Status = &status
	}

	if rawKeyword := c.Query("q"); rawKeyword != "" {
		keyword := rawKeyword
		filter.Keyword = &keyword
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return ListBatchesFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return ListBatchesFilter{}, apperr.NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}
