package inventory_test

import (
	"context"
	"errors"

	"github.com/Mercer08572/stock-flow/internal/inventory"
)

var errFakeRepositoryNotImplemented = errors.New("fake repository operation is not implemented")

func (r *fakeRepository) GetBatch(context.Context, int64, int64) (*inventory.BatchInfo, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) EnsureStockRow(context.Context, int64, int64) error {
	return errFakeRepositoryNotImplemented
}

func (r *fakeRepository) LockStockRow(context.Context, int64, int64) (*inventory.StockBalance, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) AddStockOnHand(context.Context, int64, int64, string) error {
	return errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ReserveStock(context.Context, int64, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ReleaseStockReserved(context.Context, int64, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) DecreaseStockAvailable(context.Context, int64, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ConsumeStockReserved(context.Context, int64, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) InsertStockLayer(context.Context, int64, int64, *int64, string) (*inventory.StockLayer, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ListAvailableStockLayersForUpdate(context.Context, int64, int64, *int64) ([]inventory.StockLayer, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) AddStockLayerReserved(context.Context, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) DecreaseStockLayerAvailable(context.Context, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ReleaseStockLayerReserved(context.Context, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ConsumeStockLayerReserved(context.Context, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) InsertReservation(context.Context, inventory.ReservationInput) (*inventory.ReservationRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) LockReservation(context.Context, int64) (*inventory.ReservationRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) GetReservation(context.Context, int64) (*inventory.ReservationRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) AddReservationReleased(context.Context, int64, string) (*inventory.ReservationRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) AddReservationConsumed(context.Context, int64, string) (*inventory.ReservationRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) InsertReservationItem(context.Context, int64, int64, string) (*inventory.ReservationItemRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ListReservationItemsForUpdate(context.Context, int64) ([]inventory.ReservationItemRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) AddReservationItemReleased(context.Context, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) AddReservationItemConsumed(context.Context, int64, string) (bool, error) {
	return false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) InsertMovement(context.Context, inventory.MovementInput) (*inventory.Movement, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ListMovements(context.Context, inventory.ListMovementsFilter) ([]inventory.Movement, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) ClaimIdempotencyKey(context.Context, inventory.IdempotencyClaim) (*inventory.IdempotencyRecord, bool, error) {
	return nil, false, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) GetIdempotencyKey(context.Context, string, string) (*inventory.IdempotencyRecord, error) {
	return nil, errFakeRepositoryNotImplemented
}

func (r *fakeRepository) CompleteIdempotencyKey(context.Context, inventory.IdempotencyCompletion) error {
	return errFakeRepositoryNotImplemented
}
