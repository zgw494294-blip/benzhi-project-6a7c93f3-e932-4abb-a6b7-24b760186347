package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"mural-conservation-gate/internal/domain"
	"mural-conservation-gate/internal/store"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Field          string `json:"field,omitempty"`
	CurrentVersion int64  `json:"currentVersion,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string, err error, version int64) {
	item := apiError{Code: code, Message: message, CurrentVersion: version}
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		item.Field = validation.Field
		item.Message = validation.Message
	}
	writeJSON(w, status, errorEnvelope{Error: item})
}

func mapError(w http.ResponseWriter, err error, version int64) {
	switch {
	case domain.IsValidation(err):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "输入不符合业务规则", err, version)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "资源不存在", err, version)
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "version_conflict", err.Error(), err, version)
	case errors.Is(err, domain.ErrIdempotencyReuse):
		writeError(w, http.StatusConflict, "idempotency_conflict", err.Error(), err, version)
	case errors.Is(err, domain.ErrFrozen):
		writeError(w, http.StatusConflict, "case_frozen", err.Error(), err, version)
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrIncomplete):
		writeError(w, http.StatusConflict, "workflow_blocked", err.Error(), err, version)
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器处理请求失败", err, version)
	}
}

func decodeStrict(r *http.Request, target any) ([]byte, error) {
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		return nil, domain.Invalid("Content-Type", "写操作必须使用 application/json")
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return nil, domain.Invalid("body", "JSON 格式错误或包含未知字段")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, domain.Invalid("body", "请求体只能包含一个 JSON 对象")
	}
	return data, nil
}

func (s *Server) idempotent(w http.ResponseWriter, r *http.Request, body []byte, action func() (any, int, error)) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key", "写操作必须提供 Idempotency-Key", nil, 0)
		return
	}
	if len(key) > 128 {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key 长度不得超过 128", nil, 0)
		return
	}
	fingerprint := store.RequestFingerprint(r.Method, r.URL.Path, body)
	cached, err := s.workflow.Store().LookupIdempotency(r.Context(), key, fingerprint)
	if err != nil {
		mapError(w, err, 0)
		return
	}
	if cached.Found {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Idempotency-Replayed", "true")
		w.WriteHeader(cached.StatusCode)
		_, _ = w.Write(cached.Response)
		return
	}
	if err = s.workflow.Store().ReserveIdempotency(r.Context(), key, fingerprint); err != nil {
		mapError(w, err, 0)
		return
	}
	value, status, err := action()
	if err != nil {
		version := int64(0)
		if caseID := r.PathValue("caseID"); caseID != "" {
			if current, lookupErr := s.workflow.Store().GetCase(r.Context(), caseID); lookupErr == nil {
				version = current.Version
			}
		}
		mapError(w, err, version)
		return
	}
	response, err := json.Marshal(value)
	if err != nil {
		mapError(w, fmt.Errorf("编码响应: %w", err), 0)
		return
	}
	response = append(response, '\n')
	if err = s.workflow.Store().SaveIdempotency(r.Context(), key, fingerprint, response, status); err != nil {
		mapError(w, err, 0)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(response)
}
