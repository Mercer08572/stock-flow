package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password string, encodedHash string) (bool, error)
}

type Argon2idParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

type argon2idPasswordHasher struct {
	params Argon2idParams
}

func NewArgon2idPasswordHasher(params Argon2idParams) PasswordHasher {
	if params.Memory == 0 {
		params.Memory = 64 * 1024
	}
	if params.Iterations == 0 {
		params.Iterations = 3
	}
	if params.Parallelism == 0 {
		params.Parallelism = 2
	}
	if params.SaltLength == 0 {
		params.SaltLength = 16
	}
	if params.KeyLength == 0 {
		params.KeyLength = 32
	}

	return &argon2idPasswordHasher{params: params}
}

func (h *argon2idPasswordHasher) Hash(password string) (string, error) {
	if password == "" {
		return "", apperr.NewValidationError("password is required")
	}

	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, h.params.Iterations, h.params.Memory, h.params.Parallelism, h.params.KeyLength)
	return encodeArgon2id(h.params, salt, hash), nil
}

func (h *argon2idPasswordHasher) Verify(password string, encodedHash string) (bool, error) {
	params, salt, expected, err := decodeArgon2id(encodedHash)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func encodeArgon2id(params Argon2idParams, salt []byte, hash []byte) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func decodeArgon2id(encodedHash string) (Argon2idParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Argon2idParams{}, nil, nil, errors.New("invalid Argon2id encoded hash")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Argon2idParams{}, nil, nil, errors.New("unsupported Argon2id version")
	}

	var params Argon2idParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Iterations, &params.Parallelism); err != nil {
		return Argon2idParams{}, nil, nil, errors.New("invalid Argon2id parameters")
	}
	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 {
		return Argon2idParams{}, nil, nil, errors.New("invalid Argon2id parameters")
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Argon2idParams{}, nil, nil, errors.New("invalid Argon2id salt")
	}

	hash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(hash) == 0 {
		return Argon2idParams{}, nil, nil, errors.New("invalid Argon2id hash")
	}

	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(hash))
	return params, salt, hash, nil
}
