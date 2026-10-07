package onemax

import (
	"context"
	"net/http"
	"testing"
)

func TestUnknownReasonIsKept(t *testing.T) {
	client, _ := stub(t, http.StatusOK, `{"active":true,"usable":false,"reason":"paused_by_member","usage_id":null,"daily_limit":1,"new_field":{"a":1}}`)
	got, err := client.VerifyCode(context.Background(), "123456")
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	if got.Reason != Reason("paused_by_member") {
		t.Errorf("Reason = %q", got.Reason)
	}
	if !got.Active || got.Usable || got.DailyLimit != 1 {
		t.Errorf("Verification = %+v", got)
	}
}

func TestUnknownStatusIsKept(t *testing.T) {
	client, _ := stub(t, http.StatusOK, `{"usage_id":"u1","status":"refunded","reference":null,"created_at":"2026-10-07T17:16:11Z","confirmed_at":null,"voided_at":null}`)
	got, err := client.Usage(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if got.Status != UsageStatus("refunded") {
		t.Errorf("Status = %q", got.Status)
	}
}

func TestReasonAndStatusValues(t *testing.T) {
	reasons := map[Reason]string{
		ReasonOK:               "ok",
		ReasonNotSubscribed:    "not_subscribed",
		ReasonLimitReached:     "limit_reached",
		ReasonOfferUnavailable: "offer_unavailable",
		ReasonInvalidCode:      "invalid_code",
		ReasonNotLinked:        "not_linked",
	}
	if len(reasons) != 6 {
		t.Errorf("reasons collide: %v", reasons)
	}
	for reason, want := range reasons {
		if string(reason) != want {
			t.Errorf("reason = %q, want %q", reason, want)
		}
	}
	statuses := map[UsageStatus]string{
		UsageStatusPending:   "pending",
		UsageStatusConfirmed: "confirmed",
		UsageStatusVoided:    "voided",
	}
	if len(statuses) != 3 {
		t.Errorf("statuses collide: %v", statuses)
	}
	for status, want := range statuses {
		if string(status) != want {
			t.Errorf("status = %q, want %q", status, want)
		}
	}
}
