package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound         = errors.New("资源不存在")
	ErrConflict         = errors.New("版本冲突")
	ErrFrozen           = errors.New("案卷已冻结，禁止修改")
	ErrInvalidState     = errors.New("当前状态不允许此操作")
	ErrIncomplete       = errors.New("证据不完整")
	ErrIdempotencyReuse = errors.New("幂等键已用于不同请求")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

func IsValidation(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}
