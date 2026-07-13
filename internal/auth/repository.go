package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	authdb "github.com/Mercer08572/stock-flow/internal/auth/db"
)

type postgresRepository struct {
	queries authdb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{queries: authdb.New(db)}
}

func (r *postgresRepository) GetAdminByUsername(ctx context.Context, username string) (*AdminUser, error) {
	row, err := r.queries.GetAdminByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}

	return &AdminUser{
		ID:           row.ID,
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
		Status:       Status(row.Status),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) ListAPIApps(ctx context.Context, filter ListFilter) ([]APIApp, error) {
	rows, err := r.queries.ListAPIApps(ctx, authdb.ListAPIAppsParams{
		Status:     nullableStatus(filter.Status),
		PageLimit:  filter.Limit,
		PageOffset: filter.Offset,
	})
	if err != nil {
		return nil, err
	}

	apps := make([]APIApp, 0, len(rows))
	for _, row := range rows {
		metadata, err := decodeMetadata(row.Metadata)
		if err != nil {
			return nil, err
		}
		apps = append(apps, APIApp{
			ID:          row.ID,
			AppID:       row.AppID,
			Name:        row.Name,
			Description: row.Description,
			Status:      Status(row.Status),
			Metadata:    metadata,
			CreatedAt:   row.CreatedAt.Time,
			UpdatedAt:   row.UpdatedAt.Time,
		})
	}
	return apps, nil
}

func (r *postgresRepository) GetAPIAppByID(ctx context.Context, id int64) (*APIApp, error) {
	row, err := r.queries.GetAPIAppByID(ctx, id)
	if err != nil {
		return nil, mapAPIAppError(err)
	}
	metadata, err := decodeMetadata(row.Metadata)
	if err != nil {
		return nil, err
	}
	return &APIApp{
		ID:          row.ID,
		AppID:       row.AppID,
		Name:        row.Name,
		Description: row.Description,
		Status:      Status(row.Status),
		Metadata:    metadata,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) GetAPIAppByIdentifier(ctx context.Context, appID string) (*APIApp, error) {
	row, err := r.queries.GetAPIAppByIdentifier(ctx, appID)
	if err != nil {
		return nil, mapAPIAppError(err)
	}
	metadata, err := decodeMetadata(row.Metadata)
	if err != nil {
		return nil, err
	}
	return &APIApp{
		ID:          row.ID,
		AppID:       row.AppID,
		Name:        row.Name,
		Description: row.Description,
		Status:      Status(row.Status),
		Metadata:    metadata,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) CreateAPIApp(ctx context.Context, appID string, input CreateAPIAppInput) (*APIApp, error) {
	metadata, err := encodeMetadata(input.Metadata)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateAPIApp(ctx, authdb.CreateAPIAppParams{
		AppID:       appID,
		Name:        input.Name,
		Description: input.Description,
		Status:      string(input.Status),
		Metadata:    metadata,
	})
	if err != nil {
		return nil, mapAPIAppError(err)
	}
	decoded, err := decodeMetadata(row.Metadata)
	if err != nil {
		return nil, err
	}
	return &APIApp{
		ID:          row.ID,
		AppID:       row.AppID,
		Name:        row.Name,
		Description: row.Description,
		Status:      Status(row.Status),
		Metadata:    decoded,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) UpdateAPIApp(ctx context.Context, input UpdateAPIAppInput) (*APIApp, error) {
	metadata, err := encodeMetadata(input.Metadata)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdateAPIApp(ctx, authdb.UpdateAPIAppParams{
		ID:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Status:      string(input.Status),
		Metadata:    metadata,
	})
	if err != nil {
		return nil, mapAPIAppError(err)
	}
	decoded, err := decodeMetadata(row.Metadata)
	if err != nil {
		return nil, err
	}
	return &APIApp{
		ID:          row.ID,
		AppID:       row.AppID,
		Name:        row.Name,
		Description: row.Description,
		Status:      Status(row.Status),
		Metadata:    decoded,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) SoftDeleteAPIApp(ctx context.Context, id int64) error {
	rows, err := r.queries.SoftDeleteAPIApp(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAPIAppNotFound
	}
	return nil
}

func (r *postgresRepository) CreateAPISecret(ctx context.Context, secretID string, secretHash string, input IssueSecretInput) (*APISecret, error) {
	metadata, err := encodeMetadata(input.BoundMetadata)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateAPISecret(ctx, authdb.CreateAPISecretParams{
		ApiAppID:      input.APIAppID,
		SecretID:      secretID,
		SecretHash:    secretHash,
		Name:          input.Name,
		BoundMetadata: metadata,
		ExpiresAt:     nullableTime(input.ExpiresAt),
	})
	if err != nil {
		return nil, mapAPISecretError(err)
	}
	decoded, err := decodeMetadata(row.BoundMetadata)
	if err != nil {
		return nil, err
	}
	return &APISecret{
		ID:            row.ID,
		APIAppID:      row.ApiAppID,
		SecretID:      row.SecretID,
		Name:          row.Name,
		Status:        SecretStatus(row.Status),
		BoundMetadata: decoded,
		ExpiresAt:     timePointer(row.ExpiresAt),
		LastUsedAt:    timePointer(row.LastUsedAt),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) ListAPISecrets(ctx context.Context, apiAppID int64) ([]APISecret, error) {
	rows, err := r.queries.ListAPISecrets(ctx, apiAppID)
	if err != nil {
		return nil, err
	}
	secrets := make([]APISecret, 0, len(rows))
	for _, row := range rows {
		metadata, err := decodeMetadata(row.BoundMetadata)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, APISecret{
			ID:            row.ID,
			APIAppID:      row.ApiAppID,
			SecretID:      row.SecretID,
			Name:          row.Name,
			Status:        SecretStatus(row.Status),
			BoundMetadata: metadata,
			ExpiresAt:     timePointer(row.ExpiresAt),
			LastUsedAt:    timePointer(row.LastUsedAt),
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		})
	}
	return secrets, nil
}

func (r *postgresRepository) ListAPISecretsForAuthentication(ctx context.Context, apiAppID int64) ([]APISecret, error) {
	rows, err := r.queries.ListAPISecretsForAuthentication(ctx, apiAppID)
	if err != nil {
		return nil, err
	}
	secrets := make([]APISecret, 0, len(rows))
	for _, row := range rows {
		metadata, err := decodeMetadata(row.BoundMetadata)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, APISecret{
			ID:            row.ID,
			APIAppID:      row.ApiAppID,
			SecretID:      row.SecretID,
			SecretHash:    row.SecretHash,
			Name:          row.Name,
			Status:        SecretStatus(row.Status),
			BoundMetadata: metadata,
			ExpiresAt:     timePointer(row.ExpiresAt),
			LastUsedAt:    timePointer(row.LastUsedAt),
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		})
	}
	return secrets, nil
}

func (r *postgresRepository) BlockAPISecret(ctx context.Context, apiAppID int64, secretID string) (*APISecret, error) {
	row, err := r.queries.BlockAPISecret(ctx, authdb.BlockAPISecretParams{ApiAppID: apiAppID, SecretID: secretID})
	if err != nil {
		return nil, mapAPISecretError(err)
	}
	metadata, err := decodeMetadata(row.BoundMetadata)
	if err != nil {
		return nil, err
	}
	return &APISecret{
		ID:            row.ID,
		APIAppID:      row.ApiAppID,
		SecretID:      row.SecretID,
		Name:          row.Name,
		Status:        SecretStatus(row.Status),
		BoundMetadata: metadata,
		ExpiresAt:     timePointer(row.ExpiresAt),
		LastUsedAt:    timePointer(row.LastUsedAt),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) TouchAPISecretLastUsed(ctx context.Context, id int64, usedAt time.Time) error {
	rows, err := r.queries.TouchAPISecretLastUsed(ctx, authdb.TouchAPISecretLastUsedParams{
		ID: id,
		LastUsedAt: pgtype.Timestamptz{
			Time:  usedAt,
			Valid: true,
		},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAPISecretNotFound
	}
	return nil
}

func mapAPIAppError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAPIAppNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "ux_api_apps_app_id" {
		return ErrDuplicateAppID
	}
	return err
}

func mapAPISecretError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAPISecretNotFound
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code == "23505" && pgErr.ConstraintName == "ux_api_secrets_secret_id" {
		return ErrDuplicateSecretID
	}
	if pgErr.Code == "23503" {
		return ErrAPIAppNotFound
	}
	return err
}

func encodeMetadata(metadata Metadata) ([]byte, error) {
	encoded, err := json.Marshal(normalizeMetadata(metadata))
	if err != nil {
		return nil, fmt.Errorf("encode auth metadata: %w", err)
	}
	return encoded, nil
}

func decodeMetadata(encoded []byte) (Metadata, error) {
	if len(encoded) == 0 {
		return Metadata{}, nil
	}
	var metadata Metadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return nil, fmt.Errorf("decode auth metadata: %w", err)
	}
	return normalizeMetadata(metadata), nil
}

func nullableStatus(value *Status) *string {
	if value == nil {
		return nil
	}
	status := string(*value)
	return &status
}

func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}
