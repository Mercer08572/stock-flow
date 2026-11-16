package inventory

import (
	"context"
	"errors"
	"strings"

	sku "github.com/Mercer08572/stock-flow/internal/sku"
	warehouse "github.com/Mercer08572/stock-flow/internal/warehouse"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func normalizeOperationContext(input OperationContext) (OperationContext, error) {
	idempotencyKey, err := normalizeIdempotencyKey(input.IdempotencyKey)
	if err != nil {
		return OperationContext{}, err
	}
	input.IdempotencyKey = idempotencyKey

	if input.SourceType, err = normalizeOptionalText(input.SourceType, "source_type", 64); err != nil {
		return OperationContext{}, err
	}
	if input.SourceID, err = normalizeOptionalText(input.SourceID, "source_id", 128); err != nil {
		return OperationContext{}, err
	}
	if input.SourceLineID, err = normalizeOptionalText(input.SourceLineID, "source_line_id", 128); err != nil {
		return OperationContext{}, err
	}

	if input.Actor != nil {
		actorType := strings.TrimSpace(input.Actor.Type)
		if actorType != "admin" && actorType != "api_app" {
			return OperationContext{}, apperr.NewValidationError("actor type is invalid")
		}
		if input.Actor.ID <= 0 {
			return OperationContext{}, apperr.NewValidationError("actor id must be greater than zero")
		}
		label := strings.TrimSpace(input.Actor.Label)
		if label == "" || len(label) > 128 {
			return OperationContext{}, apperr.NewValidationError("actor label must be between 1 and 128 characters")
		}
		input.Actor = &Actor{Type: actorType, ID: input.Actor.ID, Label: label}
	}

	return input, nil
}

func normalizeOptionalText(value *string, field string, maxLength int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if len(trimmed) > maxLength {
		return nil, apperr.NewValidationError(field + " is too long")
	}
	return &trimmed, nil
}

func (s *service) validateWarehouseForMutation(ctx context.Context, warehouseID int64) error {
	if warehouseID <= 0 {
		return apperr.NewValidationError("warehouse_id must be greater than zero")
	}

	reference, err := s.warehouseReader.Get(ctx, warehouseID)
	if err != nil {
		if errors.Is(err, warehouse.ErrNotFound) {
			return ErrWarehouseNotFound
		}
		if apperr.IsValidationError(err) {
			return apperr.NewValidationError(err.Error())
		}
		return err
	}
	if reference.Status != warehouse.StatusActive {
		return ErrWarehouseInactive
	}

	return nil
}

func (s *service) validateSKUForMutation(ctx context.Context, skuID int64) error {
	if skuID <= 0 {
		return apperr.NewValidationError("sku_id must be greater than zero")
	}

	if _, err := s.skuReader.Get(ctx, skuID); err != nil {
		if errors.Is(err, sku.ErrNotFound) {
			return ErrSKUNotFound
		}
		if apperr.IsValidationError(err) {
			return apperr.NewValidationError(err.Error())
		}
		return err
	}

	return nil
}

// validateBatchForMutation 校验写操作引用的批次。
//
// requireActive 只在「新增收货」与「新预留」时为 true：停用批次不再参与常规流转；
// 直接从可用库存扣减时允许显式指定停用批次，便于人工清理存量。
func (s *service) validateBatchForMutation(ctx context.Context, batchID *int64, skuID int64, requireActive bool) error {
	if batchID == nil {
		return nil
	}
	if *batchID <= 0 {
		return apperr.NewValidationError("batch_id must be greater than zero")
	}

	batch, err := s.repo.GetBatch(ctx, *batchID, skuID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrBatchReferenceInvalid
		}
		return err
	}
	if requireActive && batch.Status != "active" {
		return ErrBatchInactive
	}

	return nil
}
