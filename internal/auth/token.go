package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TokenPrefix marks Lookfar API tokens so they are easy to spot in logs and
// secret scanners.
const TokenPrefix = "lf_"

// NewToken returns a fresh API token and the hash to store for it.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is the SHA-256 digest stored in api_tokens.token_hash. Tokens are
// 256 random bits, so a fast hash is enough.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (a *Authenticator) authenticateToken(ctx context.Context, header string) (User, error) {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || !strings.HasPrefix(token, TokenPrefix) {
		return User{}, ErrInvalidSession
	}

	row, err := a.db.GetTokenUser(ctx, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	if err != nil {
		return User{}, err
	}
	if err := a.db.TouchToken(ctx, row.TokenID); err != nil {
		slog.WarnContext(ctx, "touch api token", "err", err)
	}

	return User{
		ID:            row.ID,
		Name:          row.Name,
		Email:         row.Email,
		EmailVerified: row.EmailVerified,
		Image:         row.Image,
		CreatedAt:     row.CreatedAt.UTC(),
	}, nil
}
