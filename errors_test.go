package onemax

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

var sentinels = []error{
	ErrAuthentication,
	ErrAccessDisabled,
	ErrNotFound,
	ErrConflict,
	ErrInvalidRequest,
	ErrRateLimited,
	ErrServer,
}

func envelope(code string) string {
	return fmt.Sprintf(`{"error":{"code":%q,"message":"İstifadə tapılmadı","details":{"field":"usage_id"},"request_id":"req-1"}}`, code)
}

func assertOnlySentinel(t *testing.T, err error, want error) {
	t.Helper()
	for _, sentinel := range sentinels {
		if got := errors.Is(err, sentinel); got != (sentinel == want) {
			t.Errorf("errors.Is(err, %v) = %v", sentinel, got)
		}
	}
}

func TestStatusMapsToSentinel(t *testing.T) {
	cases := []struct {
		status   int
		code     string
		sentinel error
	}{
		{http.StatusBadRequest, "offer_required", ErrInvalidRequest},
		{http.StatusUnauthorized, "invalid_api_key", ErrAuthentication},
		{http.StatusForbidden, "api_access_disabled", ErrAccessDisabled},
		{http.StatusNotFound, "usage_not_found", ErrNotFound},
		{http.StatusConflict, "reference_mismatch", ErrConflict},
		{http.StatusConflict, "a_code_added_later", ErrConflict},
		{http.StatusUnprocessableEntity, "validation_error", ErrInvalidRequest},
		{http.StatusTooManyRequests, "rate_limited", ErrRateLimited},
		{http.StatusInternalServerError, "internal_error", ErrServer},
		{http.StatusServiceUnavailable, "unavailable", ErrServer},
		{http.StatusTeapot, "teapot", nil},
		{http.StatusMethodNotAllowed, "method_not_allowed", nil},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d %s", tc.status, tc.code), func(t *testing.T) {
			client, _ := stub(t, tc.status, envelope(tc.code))
			result, err := client.Usage(context.Background(), "u1")
			if result != nil {
				t.Errorf("result = %+v, want nil", result)
			}
			var apiError *APIError
			if !errors.As(err, &apiError) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiError.Status != tc.status {
				t.Errorf("Status = %d, want %d", apiError.Status, tc.status)
			}
			if apiError.Code != tc.code {
				t.Errorf("Code = %q, want %q", apiError.Code, tc.code)
			}
			if apiError.Message != "İstifadə tapılmadı" {
				t.Errorf("Message = %q", apiError.Message)
			}
			if !reflect.DeepEqual(apiError.Details, map[string]any{"field": "usage_id"}) {
				t.Errorf("Details = %v", apiError.Details)
			}
			if apiError.RequestID != "req-1" {
				t.Errorf("RequestID = %q", apiError.RequestID)
			}
			if apiError.RetryAfter != nil {
				t.Errorf("RetryAfter = %d, want nil", *apiError.RetryAfter)
			}
			assertOnlySentinel(t, err, tc.sentinel)
			var transportError *TransportError
			if errors.As(err, &transportError) {
				t.Errorf("err also matches *TransportError")
			}
		})
	}
}

func TestRateLimitExposesRetryAfter(t *testing.T) {
	client, _ := stub(t, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"Çox sorğu","details":{"retry_after":17},"request_id":"req-2"}}`)
	_, err := client.VerifyCode(context.Background(), "123456")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err does not match ErrRateLimited")
	}
	if apiError.RetryAfter == nil || *apiError.RetryAfter != 17 {
		t.Errorf("RetryAfter = %v, want 17", apiError.RetryAfter)
	}
}

func TestRateLimitWithoutRetryAfter(t *testing.T) {
	client, _ := stub(t, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"Çox sorğu","details":{},"request_id":null}}`)
	_, err := client.VerifyCode(context.Background(), "123456")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiError.RetryAfter != nil {
		t.Errorf("RetryAfter = %d, want nil", *apiError.RetryAfter)
	}
	if apiError.RequestID != "" {
		t.Errorf("RequestID = %q, want empty", apiError.RequestID)
	}
	if apiError.Details == nil || len(apiError.Details) != 0 {
		t.Errorf("Details = %v, want an empty map", apiError.Details)
	}
}

func TestNonJSONErrorBodyStillMapsByStatus(t *testing.T) {
	cases := []struct {
		status   int
		body     string
		sentinel error
	}{
		{http.StatusBadGateway, "<html>Bad Gateway</html>", ErrServer},
		{http.StatusNotFound, "not here", ErrNotFound},
		{http.StatusUnauthorized, `{"detail":"no envelope"}`, ErrAuthentication},
		{http.StatusConflict, `{"error":"a string"}`, ErrConflict},
		{http.StatusTooManyRequests, "", ErrRateLimited},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d", tc.status), func(t *testing.T) {
			client, _ := stub(t, tc.status, tc.body)
			_, err := client.Context(context.Background())
			var apiError *APIError
			if !errors.As(err, &apiError) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiError.Status != tc.status {
				t.Errorf("Status = %d, want %d", apiError.Status, tc.status)
			}
			if apiError.Code != "unknown_error" {
				t.Errorf("Code = %q, want unknown_error", apiError.Code)
			}
			if apiError.Message != tc.body {
				t.Errorf("Message = %q, want %q", apiError.Message, tc.body)
			}
			if apiError.Details == nil || len(apiError.Details) != 0 {
				t.Errorf("Details = %v, want an empty map", apiError.Details)
			}
			if apiError.RequestID != "" {
				t.Errorf("RequestID = %q, want empty", apiError.RequestID)
			}
			assertOnlySentinel(t, err, tc.sentinel)
		})
	}
}

func TestRawErrorTextIsTruncated(t *testing.T) {
	client, _ := stub(t, http.StatusInternalServerError, strings.Repeat("ə", 500))
	_, err := client.Context(context.Background())
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiError.Message != strings.Repeat("ə", 200) {
		t.Errorf("Message has %d characters, want 200", len([]rune(apiError.Message)))
	}
}

func TestEnvelopeWithUnusualDetailsKeepsTheCode(t *testing.T) {
	client, _ := stub(t, http.StatusUnprocessableEntity, `{"error":{"code":"validation_error","message":"Yanlış sorğu","details":[{"loc":["body","code"]}],"request_id":"req-3"}}`)
	_, err := client.VerifyCode(context.Background(), "")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiError.Code != "validation_error" || apiError.RequestID != "req-3" {
		t.Errorf("APIError = %+v", apiError)
	}
	if apiError.Details == nil || len(apiError.Details) != 0 {
		t.Errorf("Details = %v, want an empty map", apiError.Details)
	}
}

func TestAPIErrorText(t *testing.T) {
	err := &APIError{Status: 409, Code: "too_late", Message: "Gecdir", RequestID: "req-9"}
	if got := err.Error(); got != "onemax: 409 too_late: Gecdir (request req-9)" {
		t.Errorf("Error() = %q", got)
	}
	err.RequestID = ""
	if got := err.Error(); got != "onemax: 409 too_late: Gecdir" {
		t.Errorf("Error() = %q", got)
	}
}

func TestAPIErrorMatchesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("checking the member: %w", &APIError{Status: http.StatusConflict, Code: "too_late"})
	if !errors.Is(wrapped, ErrConflict) {
		t.Errorf("wrapped error does not match ErrConflict")
	}
	if errors.Is(wrapped, ErrNotFound) {
		t.Errorf("wrapped error matches ErrNotFound")
	}
}

func TestTransportErrorWrapsItsCause(t *testing.T) {
	cause := errors.New("connection refused")
	err := &TransportError{Message: "the request failed", Err: cause}
	if !errors.Is(err, cause) {
		t.Errorf("err does not wrap its cause")
	}
	if got := err.Error(); got != "onemax: the request failed: connection refused" {
		t.Errorf("Error() = %q", got)
	}
	bare := &TransportError{Message: "the request failed"}
	if got := bare.Error(); got != "onemax: the request failed" {
		t.Errorf("Error() = %q", got)
	}
}
