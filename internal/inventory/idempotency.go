package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

type canonicalOperationPayload struct {
	OperationType string  `json:"operation_type"`
	WarehouseID   int64   `json:"warehouse_id"`
	SKUID         int64   `json:"sku_id"`
	BatchID       *int64  `json:"batch_id"`
	ReservationID *int64  `json:"reservation_id"`
	Qty           *string `json:"qty"`
	SourceType    *string `json:"source_type"`
	SourceID      *string `json:"source_id"`
	SourceLineID  *string `json:"source_line_id"`
	ActorType     *string `json:"actor_type"`
	ActorID       *int64  `json:"actor_id"`
}

type idempotencyClaim struct {
	ID     int64
	Replay *StockOperationResult
}

func buildIdempotencyRequest(
	operationType string,
	warehouseID int64,
	skuID int64,
	batchID *int64,
	reservationID *int64,
	qty *string,
	operationContext OperationContext,
) ([]byte, string, error) {
	payload := canonicalOperationPayload{
		OperationType: operationType,
		WarehouseID:   warehouseID,
		SKUID:         skuID,
		BatchID:       batchID,
		ReservationID: reservationID,
		Qty:           qty,
		SourceType:    operationContext.SourceType,
		SourceID:      operationContext.SourceID,
		SourceLineID:  operationContext.SourceLineID,
	}
	if operationContext.Actor != nil {
		actorType := operationContext.Actor.Type
		actorID := operationContext.Actor.ID
		payload.ActorType = &actorType
		payload.ActorID = &actorID
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("marshal idempotency payload: %w", err)
	}
	sum := sha256.Sum256(encoded)

	return encoded, hex.EncodeToString(sum[:]), nil
}

func claimIdempotency(
	ctx context.Context,
	repo Repository,
	operationType string,
	operationContext OperationContext,
	requestHash string,
	requestPayload []byte,
) (*idempotencyClaim, error) {
	record, created, err := repo.ClaimIdempotencyKey(ctx, IdempotencyClaim{
		OperationType:  operationType,
		Key:            operationContext.IdempotencyKey,
		RequestHash:    requestHash,
		RequestPayload: requestPayload,
	})
	if err != nil {
		return nil, err
	}
	if created {
		return &idempotencyClaim{ID: record.ID}, nil
	}

	existing, err := repo.GetIdempotencyKey(ctx, operationType, operationContext.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing.RequestHash != requestHash {
		return nil, ErrIdempotencyConflict
	}

	switch existing.Status {
	case "succeeded":
		if len(existing.ResponsePayload) == 0 {
			return nil, errors.New("idempotency record is missing its response payload")
		}
		var replay StockOperationResult
		if err := json.Unmarshal(existing.ResponsePayload, &replay); err != nil {
			return nil, fmt.Errorf("decode idempotent response: %w", err)
		}
		return &idempotencyClaim{ID: existing.ID, Replay: &replay}, nil
	case "processing":
		return nil, ErrOperationInProgress
	default:
		return nil, ErrIdempotencyConflict
	}
}

func completeIdempotency(ctx context.Context, repo Repository, claimID int64, result *StockOperationResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}

	var (
		refType *string
		refID   *int64
	)
	switch {
	case result.Reservation != nil:
		value := "reservation"
		id := result.Reservation.ID
		refType = &value
		refID = &id
	case len(result.Movements) > 0:
		value := "movement"
		id := result.Movements[0].ID
		refType = &value
		refID = &id
	}

	return repo.CompleteIdempotencyKey(ctx, IdempotencyCompletion{
		ID:              claimID,
		ResponseRefType: refType,
		ResponseRefID:   refID,
		ResponsePayload: payload,
	})
}

func normalizeIdempotencyKey(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", apperr.NewValidationError("idempotency_key is required")
	}
	if len(trimmed) > 128 {
		return "", apperr.NewValidationError("idempotency_key must be at most 128 characters")
	}
	return trimmed, nil
}
