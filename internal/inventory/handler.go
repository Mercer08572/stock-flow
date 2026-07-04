package inventory

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
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
	stocks := router.Group("/inventory/stocks")
	stocks.GET("", h.ListStocks)
	stocks.GET("/:warehouse_id/:sku_id/layers", h.ListLayers)
	stocks.GET("/:warehouse_id/:sku_id", h.GetStock)
}

// ListStocks godoc
// @Summary List stock balances
// @Tags Inventory
// @Produce json
// @Param warehouse_id query int false "Warehouse ID"
// @Param sku_id query int false "SKU ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=StockListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/stocks [get]
func (h *handler) ListStocks(c *gin.Context) {
	filter, err := parseListStocksFilter(c)
	if err != nil {
		writeError(c, err)
		return
	}

	result, err := h.service.ListStocks(c.Request.Context(), filter)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, result)
}

// GetStock godoc
// @Summary Get a stock balance
// @Tags Inventory
// @Produce json
// @Param warehouse_id path int true "Warehouse ID"
// @Param sku_id path int true "SKU ID"
// @Param include_layers query bool false "Include stock layers"
// @Success 200 {object} response.Body{data=StockBalance}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/stocks/{warehouse_id}/{sku_id} [get]
func (h *handler) GetStock(c *gin.Context) {
	warehouseID, skuID, err := parseWarehouseAndSKUID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	includeLayers := false
	if rawIncludeLayers := c.Query("include_layers"); rawIncludeLayers != "" {
		includeLayers, err = strconv.ParseBool(rawIncludeLayers)
		if err != nil {
			writeError(c, NewValidationError("include_layers must be a boolean"))
			return
		}
	}

	stock, err := h.service.GetStock(c.Request.Context(), GetStockQuery{
		WarehouseID:   warehouseID,
		SKUID:         skuID,
		IncludeLayers: includeLayers,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, stock)
}

// ListLayers godoc
// @Summary List stock layers
// @Tags Inventory
// @Produce json
// @Param warehouse_id path int true "Warehouse ID"
// @Param sku_id path int true "SKU ID"
// @Param batch_id query int false "Batch ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=LayerListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/stocks/{warehouse_id}/{sku_id}/layers [get]
func (h *handler) ListLayers(c *gin.Context) {
	filter, err := parseListLayersFilter(c)
	if err != nil {
		writeError(c, err)
		return
	}

	result, err := h.service.ListLayers(c.Request.Context(), filter)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, result)
}

func parseListStocksFilter(c *gin.Context) (ListStocksFilter, error) {
	filter := ListStocksFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if rawWarehouseID := c.Query("warehouse_id"); rawWarehouseID != "" {
		warehouseID, err := strconv.ParseInt(rawWarehouseID, 10, 64)
		if err != nil {
			return ListStocksFilter{}, NewValidationError("warehouse_id must be an integer")
		}
		filter.WarehouseID = &warehouseID
	}

	if rawSKUID := c.Query("sku_id"); rawSKUID != "" {
		skuID, err := strconv.ParseInt(rawSKUID, 10, 64)
		if err != nil {
			return ListStocksFilter{}, NewValidationError("sku_id must be an integer")
		}
		filter.SKUID = &skuID
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return ListStocksFilter{}, NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return ListStocksFilter{}, NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}

func parseListLayersFilter(c *gin.Context) (ListLayersFilter, error) {
	warehouseID, skuID, err := parseWarehouseAndSKUID(c)
	if err != nil {
		return ListLayersFilter{}, err
	}

	filter := ListLayersFilter{
		WarehouseID: warehouseID,
		SKUID:       skuID,
		Limit:       DefaultListLimit,
		Offset:      0,
	}

	if rawBatchID := c.Query("batch_id"); rawBatchID != "" {
		batchID, err := strconv.ParseInt(rawBatchID, 10, 64)
		if err != nil {
			return ListLayersFilter{}, NewValidationError("batch_id must be an integer")
		}
		filter.BatchID = &batchID
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return ListLayersFilter{}, NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return ListLayersFilter{}, NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}

func parseWarehouseAndSKUID(c *gin.Context) (int64, int64, error) {
	warehouseID, err := strconv.ParseInt(c.Param("warehouse_id"), 10, 64)
	if err != nil || warehouseID <= 0 {
		return 0, 0, NewValidationError("warehouse_id must be greater than zero")
	}

	skuID, err := strconv.ParseInt(c.Param("sku_id"), 10, 64)
	if err != nil || skuID <= 0 {
		return 0, 0, NewValidationError("sku_id must be greater than zero")
	}

	return warehouseID, skuID, nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case IsValidationError(err), errors.Is(err, ErrWarehouseNotFound), errors.Is(err, ErrSKUNotFound), errors.Is(err, ErrBatchNotFound):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
