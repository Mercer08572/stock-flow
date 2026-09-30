package httperr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

type envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code"`
}

func write(t *testing.T, err error) (*httptest.ResponseRecorder, envelope, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/warehouses", nil)

	httperr.Write(c, err)

	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return rec, body, c
}

func TestWriteMapsBusinessErrorStatusAndCode(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int
		wantCodeID apperr.Code
	}{
		{
			name:       "conflict",
			err:        apperr.Conflict(apperr.CodeWarehouseCodeDuplicate, "warehouse code already exists"),
			wantStatus: http.StatusConflict,
			wantCode:   response.CodeConflict,
			wantCodeID: apperr.CodeWarehouseCodeDuplicate,
		},
		{
			name:       "not found",
			err:        apperr.NotFound(apperr.CodeWarehouseNotFound, "warehouse not found"),
			wantStatus: http.StatusNotFound,
			wantCode:   response.CodeNotFound,
			wantCodeID: apperr.CodeWarehouseNotFound,
		},
		{
			name:       "unauthorized",
			err:        apperr.Unauthorized(apperr.CodeAuthInvalidCredentials, "authentication failed"),
			wantStatus: http.StatusUnauthorized,
			wantCode:   response.CodeUnauthorized,
			wantCodeID: apperr.CodeAuthInvalidCredentials,
		},
		{
			name:       "forbidden",
			err:        apperr.Forbidden(apperr.CodeAuthPasswordChangeRequired, "administrator password change required"),
			wantStatus: http.StatusForbidden,
			wantCode:   response.CodeForbidden,
			wantCodeID: apperr.CodeAuthPasswordChangeRequired,
		},
		{
			name:       "bad request with business code",
			err:        apperr.BadRequest(apperr.CodeSKUUnitNotAllowed, "sku unit is not allowed for material"),
			wantStatus: http.StatusBadRequest,
			wantCode:   response.CodeBadRequest,
			wantCodeID: apperr.CodeSKUUnitNotAllowed,
		},
		{
			// 429 沿用 1001 是既有行为，这里刻意锁定，避免被「顺手修正」。
			name:       "too many requests keeps legacy numeric code",
			err:        apperr.TooManyRequests(apperr.CodeAuthRateLimited, "too many login attempts"),
			wantStatus: http.StatusTooManyRequests,
			wantCode:   response.CodeBadRequest,
			wantCodeID: apperr.CodeAuthRateLimited,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body, _ := write(t, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}
			if body.Code != tc.wantCode {
				t.Fatalf("expected numeric code %d, got %d", tc.wantCode, body.Code)
			}
			if body.ErrorCode != string(tc.wantCodeID) {
				t.Fatalf("expected error code %q, got %q", tc.wantCodeID, body.ErrorCode)
			}
			if body.Message != tc.err.Error() {
				t.Fatalf("expected message %q, got %q", tc.err.Error(), body.Message)
			}
		})
	}
}

func TestWriteMapsWrappedBusinessError(t *testing.T) {
	wrapped := fmt.Errorf("delete warehouse: %w", apperr.Conflict(apperr.CodeWarehouseReferencedByInventory, "warehouse is referenced by inventory and cannot be deleted"))

	rec, body, _ := write(t, wrapped)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if body.ErrorCode != string(apperr.CodeWarehouseReferencedByInventory) {
		t.Fatalf("expected error code %q, got %q", apperr.CodeWarehouseReferencedByInventory, body.ErrorCode)
	}
}

func TestWriteMapsValidationErrorWithoutBusinessCode(t *testing.T) {
	rec, body, _ := write(t, apperr.NewValidationError("code is required"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if body.Code != response.CodeBadRequest {
		t.Fatalf("expected numeric code %d, got %d", response.CodeBadRequest, body.Code)
	}
	if body.Message != "code is required" {
		t.Fatalf("expected validation message, got %q", body.Message)
	}
	if strings.Contains(rec.Body.String(), "error_code") {
		t.Fatalf("validation errors must not carry a business code: %s", rec.Body.String())
	}
}

func TestWriteHidesUnknownErrorButKeepsCause(t *testing.T) {
	cause := errors.New("pgx: connection refused")

	rec, body, c := write(t, cause)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if body.Message != "internal server error" {
		t.Fatalf("expected generic message, got %q", body.Message)
	}
	if strings.Contains(rec.Body.String(), "pgx") {
		t.Fatalf("internal cause leaked into response: %s", rec.Body.String())
	}
	if len(c.Errors) != 1 || !errors.Is(c.Errors[0], cause) {
		t.Fatalf("expected the real cause to be attached to gin's error chain, got %+v", c.Errors)
	}
}

// headeredError 模拟 auth 的限流错误：包装一个业务错误，并额外携带响应头。
type headeredError struct {
	err error
}

func (e *headeredError) Error() string { return e.err.Error() }
func (e *headeredError) Unwrap() error { return e.err }

func (e *headeredError) Headers() map[string]string {
	return map[string]string{"Retry-After": "42"}
}

func TestWriteAppliesErrorHeaders(t *testing.T) {
	rec, body, _ := write(t, &headeredError{
		err: apperr.TooManyRequests(apperr.CodeAuthRateLimited, "too many login attempts"),
	})

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "42" {
		t.Fatalf("expected Retry-After header %q, got %q", "42", got)
	}
	if body.ErrorCode != string(apperr.CodeAuthRateLimited) {
		t.Fatalf("expected error code %q, got %q", apperr.CodeAuthRateLimited, body.ErrorCode)
	}
}
