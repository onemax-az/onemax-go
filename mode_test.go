package onemax

import (
	"context"
	"net/http"
	"testing"
)

func TestClientKnowsTheModeOfItsKey(t *testing.T) {
	cases := map[string]Mode{
		"omx_test_abc":     ModeTest,
		"  omx_test_abc  ": ModeTest,
		"omx_live_abc":     ModeLive,
		"omx_other_abc":    ModeLive,
	}
	for key, want := range cases {
		client, err := NewClient(key)
		if err != nil {
			t.Fatalf("NewClient(%q): %v", key, err)
		}
		if got := client.Mode(); got != want {
			t.Errorf("Mode(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestEveryResultCarriesTheModeTheAPIAnsweredIn(t *testing.T) {
	ctx := context.Background()
	calls := []struct {
		name string
		body string
		call func(*Client) (Mode, error)
	}{
		{"context", `"partner_name":"Taksi","venue_id":"v","venue_name":"V","daily_limit":1,"offers":[]`, func(c *Client) (Mode, error) {
			got, err := c.Context(ctx)
			if err != nil {
				return "", err
			}
			return got.Mode, nil
		}},
		{"verify", `"active":true,"usable":true,"reason":"ok","usage_id":"u1","daily_limit":1`, func(c *Client) (Mode, error) {
			got, err := c.VerifyCode(ctx, "000001")
			if err != nil {
				return "", err
			}
			return got.Mode, nil
		}},
		{"usage", `"usage_id":"u1","status":"pending","reference":null,"created_at":"2026-10-19T10:00:00Z","confirmed_at":null,"voided_at":null`, func(c *Client) (Mode, error) {
			got, err := c.Usage(ctx, "u1")
			if err != nil {
				return "", err
			}
			return got.Mode, nil
		}},
		{"token", `"link_token":"omx_link_test_x","active":true`, func(c *Client) (Mode, error) {
			got, err := c.ExchangeCode(ctx, "c", "v", "https://a.example")
			if err != nil {
				return "", err
			}
			return got.Mode, nil
		}},
		{"status", `"linked":true,"active":false`, func(c *Client) (Mode, error) {
			got, err := c.LinkStatus(ctx, "omx_link_test_x")
			if err != nil {
				return "", err
			}
			return got.Mode, nil
		}},
	}
	answers := map[string]Mode{`,"mode":"test"`: ModeTest, `,"mode":"live"`: ModeLive, `,"mode":"staging"`: Mode("staging"), ``: Mode("")}
	for _, call := range calls {
		for extra, want := range answers {
			client, _ := stub(t, http.StatusOK, "{"+call.body+extra+"}")
			got, err := call.call(client)
			if err != nil {
				t.Fatalf("%s: %v", call.name, err)
			}
			if got != want {
				t.Errorf("%s with %q: Mode = %q, want %q", call.name, extra, got, want)
			}
		}
	}
}
