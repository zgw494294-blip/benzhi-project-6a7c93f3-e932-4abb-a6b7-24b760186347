package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"mural-conservation-gate/internal/domain"
)

type IdempotencyResult struct {
	Found      bool
	Response   []byte
	StatusCode int
}

func RequestFingerprint(method, path string, body []byte) string {
	sum := sha256.Sum256(append(append([]byte(method+"\n"+path+"\n"), body...), '\n'))
	return hex.EncodeToString(sum[:])
}

func (s *Store) LookupIdempotency(ctx context.Context, key, fingerprint string) (IdempotencyResult, error) {
	var stored string
	var result IdempotencyResult
	err := s.db.QueryRowContext(ctx, `SELECT request_fingerprint,response,status_code FROM idempotency_records WHERE idempotency_key=?`, key).Scan(&stored, &result.Response, &result.StatusCode)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if stored != fingerprint {
		return result, domain.ErrIdempotencyReuse
	}
	result.Found = true
	return result, nil
}

func (s *Store) SaveIdempotency(ctx context.Context, key, fingerprint string, response []byte, status int) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO idempotency_records(idempotency_key,request_fingerprint,response,status_code,created_at) VALUES(?,?,?,?,?)`, key, fingerprint, response, status, stamp(time.Now().UTC()))
	return err
}
