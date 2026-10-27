package inventory

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

// ListReservations godoc
// @Summary List inventory reservations
// @Tags Inventory
// @Produce json
// @Param warehouse_id query int false "Warehouse ID"
// @Param sku_id query int false "SKU ID"
// @Param status query string false "Reservation status (active or closed)"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=ReservationListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/reservations [get]
func (h *handler) ListReservations(c *gin.Context) {
	filter, err := parseListReservationsFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	result, err := h.service.ListReservations(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// GetReservation godoc
// @Summary Get an inventory reservation
// @Tags Inventory
// @Produce json
// @Param id path int true "Reservation ID"
// @Success 200 {object} response.Body{data=Reservation}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /inventory/reservations/{id} [get]
func (h *handler) GetReservation(c *gin.Context) {
	reservationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httperr.Write(c, apperr.NewValidationError("id must be an integer"))
		return
	}

	reservation, err := h.service.GetReservation(c.Request.Context(), reservationID)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, reservation)
}

func parseListReservationsFilter(c *gin.Context) (ListReservationsFilter, error) {
	filter := ListReservationsFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if rawWarehouseID := c.Query("warehouse_id"); rawWarehouseID != "" {
		warehouseID, err := strconv.ParseInt(rawWarehouseID, 10, 64)
		if err != nil {
			return ListReservationsFilter{}, apperr.NewValidationError("warehouse_id must be an integer")
		}
		filter.WarehouseID = &warehouseID
	}

	if rawSKUID := c.Query("sku_id"); rawSKUID != "" {
		skuID, err := strconv.ParseInt(rawSKUID, 10, 64)
		if err != nil {
			return ListReservationsFilter{}, apperr.NewValidationError("sku_id must be an integer")
		}
		filter.SKUID = &skuID
	}

	if rawStatus := c.Query("status"); rawStatus != "" {
		status := rawStatus
		filter.Status = &status
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return ListReservationsFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return ListReservationsFilter{}, apperr.NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}
