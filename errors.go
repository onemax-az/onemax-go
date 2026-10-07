package onemax

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	unknownErrorCode   = "unknown_error"
	rawMessageMaxRunes = 200
)

var (
	ErrMissingAPIKey     = errors.New("onemax: api key is empty")
	ErrUnsupportedLocale = errors.New("onemax: unsupported locale")
	ErrAuthentication    = errors.New("onemax: authentication failed")
	ErrAccessDisabled    = errors.New("onemax: api access disabled")
	ErrNotFound          = errors.New("onemax: not found")
	ErrConflict          = errors.New("onemax: conflict")
	ErrInvalidRequest    = errors.New("onemax: invalid request")
	ErrRateLimited       = errors.New("onemax: rate limited")
	ErrServer            = errors.New("onemax: server error")
)

type APIError struct {
	Status     int
	Code       string
	Message    string
	Details    map[string]any
	RequestID  string
	RetryAfter *int
}

func (e *APIError) Error() string {
	text := fmt.Sprintf("onemax: %d %s: %s", e.Status, e.Code, e.Message)
	if e.RequestID != "" {
		text += " (request " + e.RequestID + ")"
	}
	return text
}

func (e *APIError) Is(target error) bool {
	switch target {
	case ErrAuthentication:
		return e.Status == http.StatusUnauthorized
	case ErrAccessDisabled:
		return e.Status == http.StatusForbidden
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case ErrConflict:
		return e.Status == http.StatusConflict
	case ErrInvalidRequest:
		return e.Status == http.StatusBadRequest || e.Status == http.StatusUnprocessableEntity
	case ErrRateLimited:
		return e.Status == http.StatusTooManyRequests
	case ErrServer:
		return e.Status >= http.StatusInternalServerError
	}
	return false
}

type TransportError struct {
	Message string
	Err     error
}

func (e *TransportError) Error() string {
	if e.Err == nil {
		return "onemax: " + e.Message
	}
	return "onemax: " + e.Message + ": " + e.Err.Error()
}

func (e *TransportError) Unwrap() error {
	return e.Err
}

type errorEnvelope struct {
	Error *errorBody `json:"error"`
}

type errorBody struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Details   json.RawMessage `json:"details"`
	RequestID *string         `json:"request_id"`
}

func newAPIError(status int, raw []byte) *APIError {
	var envelope errorEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Error == nil || envelope.Error.Code == "" {
		return &APIError{
			Status:  status,
			Code:    unknownErrorCode,
			Message: truncate(string(raw), rawMessageMaxRunes),
			Details: map[string]any{},
		}
	}
	apiError := &APIError{
		Status:  status,
		Code:    envelope.Error.Code,
		Message: envelope.Error.Message,
		Details: map[string]any{},
	}
	if envelope.Error.RequestID != nil {
		apiError.RequestID = *envelope.Error.RequestID
	}
	var details map[string]any
	if err := json.Unmarshal(envelope.Error.Details, &details); err == nil && details != nil {
		apiError.Details = details
	}
	if status == http.StatusTooManyRequests {
		apiError.RetryAfter = retryAfter(apiError.Details)
	}
	return apiError
}

func retryAfter(details map[string]any) *int {
	value, ok := details["retry_after"].(float64)
	if !ok {
		return nil
	}
	seconds := int(value)
	return &seconds
}

func truncate(text string, limit int) string {
	runes := []rune(strings.ToValidUTF8(text, ""))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
