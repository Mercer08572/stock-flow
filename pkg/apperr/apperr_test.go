package apperr_test

import (
	"errors"
	"testing"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestErrorReturnsMessage(t *testing.T) {
	err := apperr.NotFound(apperr.CodeWarehouseNotFound, "warehouse not found")

	if err.Error() != "warehouse not found" {
		t.Fatalf("expected message, got %q", err.Error())
	}
	if err.Status != 404 {
		t.Fatalf("expected status 404, got %d", err.Status)
	}
}

func TestIsMatchesByBusinessCode(t *testing.T) {
	sentinel := apperr.Conflict(apperr.CodeWarehouseCodeDuplicate, "warehouse code already exists")
	// 同一业务码、不同措辞、在不同位置构造：语义上是同一个错误。
	rebuilt := apperr.Conflict(apperr.CodeWarehouseCodeDuplicate, "duplicate warehouse code")

	if !errors.Is(rebuilt, sentinel) {
		t.Fatal("expected errors.Is to match errors sharing the same business code")
	}
}

func TestIsRejectsDifferentCode(t *testing.T) {
	duplicate := apperr.Conflict(apperr.CodeWarehouseCodeDuplicate, "warehouse code already exists")
	referenced := apperr.Conflict(apperr.CodeWarehouseReferencedByInventory, "...")

	if errors.Is(referenced, duplicate) {
		t.Fatal("expected errors.Is to reject a different business code")
	}
}

func TestIsRejectsEmptyCodes(t *testing.T) {
	first := apperr.New(400, "", "first")
	second := apperr.New(400, "", "second")

	if errors.Is(first, second) {
		t.Fatal("errors without a business code must not match each other")
	}
}

func TestIsSurvivesWrapping(t *testing.T) {
	sentinel := apperr.NotFound(apperr.CodeWarehouseNotFound, "warehouse not found")
	wrapped := errors.Join(errors.New("repository"), sentinel)

	if !errors.Is(wrapped, sentinel) {
		t.Fatal("expected errors.Is to find the sentinel behind a wrap")
	}
}

func TestIsValidationError(t *testing.T) {
	if !apperr.IsValidationError(apperr.NewValidationError("code is required")) {
		t.Fatal("expected a validation error to be recognized")
	}
	if apperr.IsValidationError(apperr.NotFound(apperr.CodeWarehouseNotFound, "warehouse not found")) {
		t.Fatal("business errors must not be treated as validation errors")
	}
	if apperr.IsValidationError(errors.New("boom")) {
		t.Fatal("plain errors must not be treated as validation errors")
	}
}
