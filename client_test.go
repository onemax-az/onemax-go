package onemax

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testKey      = "omx_live_test"
	usagePending = `{"usage_id":"u1","status":"pending","reference":null,"created_at":"2026-10-07T17:16:11.906636Z","confirmed_at":null,"voided_at":null}`
	usageDone    = `{"usage_id":"u1","status":"confirmed","reference":"order-7","created_at":"2026-10-07T17:16:11.906636Z","confirmed_at":"2026-10-07T21:20:00+04:00","voided_at":null}`
	usageVoided  = `{"usage_id":"u1","status":"voided","reference":null,"created_at":"2026-10-07T17:16:11.906636Z","confirmed_at":null,"voided_at":"2026-10-07T17:30:00Z"}`
	verifiedYes  = `{"active":true,"usable":true,"reason":"ok","usage_id":"u1","daily_limit":2}`
	verifiedNo   = `{"active":false,"usable":false,"reason":"not_subscribed","usage_id":null,"daily_limit":2}`
)

type exchange struct {
	Method        string
	URI           string
	Authorization string
	UserAgent     string
	Accept        string
	ContentType   string
	Body          []byte
}

type recorder struct {
	mu        sync.Mutex
	exchanges []exchange
}

func (r *recorder) add(seen exchange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchanges = append(r.exchanges, seen)
}

func (r *recorder) only(t *testing.T) exchange {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.exchanges) != 1 {
		t.Fatalf("expected exactly one request, got %d", len(r.exchanges))
	}
	return r.exchanges[0]
}

func stub(t *testing.T, status int, response string) (*Client, *recorder) {
	t.Helper()
	seen := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.add(exchange{
			Method:        r.Method,
			URI:           r.RequestURI,
			Authorization: r.Header.Get("Authorization"),
			UserAgent:     r.Header.Get("User-Agent"),
			Accept:        r.Header.Get("Accept"),
			ContentType:   r.Header.Get("Content-Type"),
			Body:          body,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(testKey, WithBaseURL(server.URL+"/v1"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, seen
}

func assertRequest(t *testing.T, seen exchange, method, uri, body string) {
	t.Helper()
	if seen.Method != method {
		t.Errorf("method = %q, want %q", seen.Method, method)
	}
	if seen.URI != uri {
		t.Errorf("uri = %q, want %q", seen.URI, uri)
	}
	if seen.Authorization != "Bearer "+testKey {
		t.Errorf("Authorization = %q", seen.Authorization)
	}
	if seen.UserAgent != "onemax-go/"+Version {
		t.Errorf("User-Agent = %q", seen.UserAgent)
	}
	if seen.Accept != "application/json" {
		t.Errorf("Accept = %q", seen.Accept)
	}
	if body == "" {
		if len(seen.Body) != 0 {
			t.Errorf("body = %q, want none", seen.Body)
		}
		if seen.ContentType != "" {
			t.Errorf("Content-Type = %q, want none", seen.ContentType)
		}
		return
	}
	if seen.ContentType != "application/json" {
		t.Errorf("Content-Type = %q", seen.ContentType)
	}
	var got, want map[string]any
	if err := json.Unmarshal(seen.Body, &got); err != nil {
		t.Fatalf("request body is not JSON: %q", seen.Body)
	}
	if err := json.Unmarshal([]byte(body), &want); err != nil {
		t.Fatalf("expected body is not JSON: %q", body)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body = %s, want %s", seen.Body, body)
	}
}

func TestVersion(t *testing.T) {
	if Version != "0.2.0" {
		t.Errorf("Version = %q", Version)
	}
}

func TestNewClientRefusesEmptyKey(t *testing.T) {
	client, err := NewClient("")
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("err = %v, want ErrMissingAPIKey", err)
	}
	if client != nil {
		t.Errorf("client = %v, want nil", client)
	}
}

func TestNewClientDefaults(t *testing.T) {
	client, err := NewClient(testKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.baseURL != "https://api.onemax.az/v1" {
		t.Errorf("baseURL = %q", client.baseURL)
	}
	if client.siteURL != "https://onemax.az" {
		t.Errorf("siteURL = %q", client.siteURL)
	}
	if client.timeout != 10*time.Second {
		t.Errorf("timeout = %v", client.timeout)
	}
}

func TestContext(t *testing.T) {
	client, seen := stub(t, http.StatusOK, `{"partner_name":"Kofe Evi","venue_id":"v1","venue_name":"Nizami","daily_limit":2,"extra":"ignored","offers":[{"id":"o1","headline":"1+1","title":"Kofe","open_now":true,"later":1},{"id":"o2","headline":"1+1","title":"Desert","open_now":false}]}`)
	got, err := client.Context(context.Background())
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodGet, "/v1/integration/context", "")
	want := &Context{
		PartnerName: "Kofe Evi",
		VenueID:     "v1",
		VenueName:   "Nizami",
		DailyLimit:  2,
		Offers: []Offer{
			{ID: "o1", Headline: "1+1", Title: "Kofe", OpenNow: true},
			{ID: "o2", Headline: "1+1", Title: "Desert", OpenNow: false},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Context = %+v, want %+v", got, want)
	}
}

func TestVerifyCodeOmitsOffer(t *testing.T) {
	client, seen := stub(t, http.StatusOK, verifiedYes)
	got, err := client.VerifyCode(context.Background(), "123456")
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/members/verify", `{"code":"123456"}`)
	if !got.Active || !got.Usable || got.Reason != ReasonOK || got.DailyLimit != 2 {
		t.Errorf("Verification = %+v", got)
	}
	if got.UsageID == nil || *got.UsageID != "u1" {
		t.Errorf("UsageID = %v, want u1", got.UsageID)
	}
}

func TestVerifyCodeWithOffer(t *testing.T) {
	client, seen := stub(t, http.StatusOK, verifiedNo)
	got, err := client.VerifyCode(context.Background(), "123456", WithOffer("o1"))
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/members/verify", `{"code":"123456","offer_id":"o1"}`)
	if got.Active || got.Usable || got.Reason != ReasonNotSubscribed {
		t.Errorf("Verification = %+v", got)
	}
	if got.UsageID != nil {
		t.Errorf("UsageID = %q, want nil", *got.UsageID)
	}
}

func TestUsage(t *testing.T) {
	client, seen := stub(t, http.StatusOK, usagePending)
	got, err := client.Usage(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodGet, "/v1/integration/usages/u1", "")
	if got.UsageID != "u1" || got.Status != UsageStatusPending {
		t.Errorf("Usage = %+v", got)
	}
	if got.Reference != nil || got.ConfirmedAt != nil || got.VoidedAt != nil {
		t.Errorf("nullable fields = %v %v %v, want nil", got.Reference, got.ConfirmedAt, got.VoidedAt)
	}
	created := time.Date(2026, time.October, 7, 17, 16, 11, 906636000, time.UTC)
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created)
	}
	if _, offset := got.CreatedAt.Zone(); offset != 0 {
		t.Errorf("CreatedAt offset = %d, want 0", offset)
	}
}

func TestConfirmUsageWithReference(t *testing.T) {
	client, seen := stub(t, http.StatusOK, usageDone)
	got, err := client.ConfirmUsage(context.Background(), "u1", WithReference("order-7"))
	if err != nil {
		t.Fatalf("ConfirmUsage: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/usages/u1/confirm", `{"reference":"order-7"}`)
	if got.Status != UsageStatusConfirmed {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Reference == nil || *got.Reference != "order-7" {
		t.Errorf("Reference = %v", got.Reference)
	}
	confirmed := time.Date(2026, time.October, 7, 17, 20, 0, 0, time.UTC)
	if got.ConfirmedAt == nil || !got.ConfirmedAt.Equal(confirmed) {
		t.Fatalf("ConfirmedAt = %v, want %v", got.ConfirmedAt, confirmed)
	}
	if _, offset := got.ConfirmedAt.Zone(); offset != 4*60*60 {
		t.Errorf("ConfirmedAt offset = %d, want 14400", offset)
	}
	if got.VoidedAt != nil {
		t.Errorf("VoidedAt = %v, want nil", got.VoidedAt)
	}
}

func TestConfirmUsageWithoutReferenceSendsEmptyObject(t *testing.T) {
	client, seen := stub(t, http.StatusOK, usageDone)
	if _, err := client.ConfirmUsage(context.Background(), "u1"); err != nil {
		t.Fatalf("ConfirmUsage: %v", err)
	}
	request := seen.only(t)
	assertRequest(t, request, http.MethodPost, "/v1/integration/usages/u1/confirm", `{}`)
	if string(request.Body) != "{}" {
		t.Errorf("body = %q, want {}", request.Body)
	}
}

func TestVoidUsageSendsNoBody(t *testing.T) {
	client, seen := stub(t, http.StatusOK, usageVoided)
	got, err := client.VoidUsage(context.Background(), "u1")
	if err != nil {
		t.Fatalf("VoidUsage: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/usages/u1/void", "")
	if got.Status != UsageStatusVoided {
		t.Errorf("Status = %q", got.Status)
	}
	voided := time.Date(2026, time.October, 7, 17, 30, 0, 0, time.UTC)
	if got.VoidedAt == nil || !got.VoidedAt.Equal(voided) {
		t.Errorf("VoidedAt = %v, want %v", got.VoidedAt, voided)
	}
}

func TestExchangeCode(t *testing.T) {
	client, seen := stub(t, http.StatusOK, `{"link_token":"omx_link_abc","active":true}`)
	got, err := client.ExchangeCode(context.Background(), "code-1", "verifier-1", "https://partner.example/callback")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/links/token", `{"code":"code-1","code_verifier":"verifier-1","redirect_uri":"https://partner.example/callback"}`)
	if got.LinkToken != "omx_link_abc" || !got.Active {
		t.Errorf("LinkToken = %+v", got)
	}
}

func TestLinkStatus(t *testing.T) {
	client, seen := stub(t, http.StatusOK, `{"linked":true,"active":false}`)
	got, err := client.LinkStatus(context.Background(), "omx_link_abc")
	if err != nil {
		t.Fatalf("LinkStatus: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/links/status", `{"link_token":"omx_link_abc"}`)
	if !got.Linked || got.Active {
		t.Errorf("LinkStatus = %+v", got)
	}
}

func TestVerifyLinkOmitsOffer(t *testing.T) {
	client, seen := stub(t, http.StatusOK, `{"active":false,"usable":false,"reason":"not_linked","usage_id":null,"daily_limit":2}`)
	got, err := client.VerifyLink(context.Background(), "omx_link_abc")
	if err != nil {
		t.Fatalf("VerifyLink: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/links/verify", `{"link_token":"omx_link_abc"}`)
	if got.Reason != ReasonNotLinked || got.UsageID != nil {
		t.Errorf("Verification = %+v", got)
	}
}

func TestVerifyLinkWithOffer(t *testing.T) {
	client, seen := stub(t, http.StatusOK, verifiedYes)
	got, err := client.VerifyLink(context.Background(), "omx_link_abc", WithOffer("o1"))
	if err != nil {
		t.Fatalf("VerifyLink: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/links/verify", `{"link_token":"omx_link_abc","offer_id":"o1"}`)
	if !got.Usable || got.UsageID == nil || *got.UsageID != "u1" {
		t.Errorf("Verification = %+v", got)
	}
}

func TestRevokeLink(t *testing.T) {
	client, seen := stub(t, http.StatusOK, `{"ok":true}`)
	if err := client.RevokeLink(context.Background(), "omx_link_abc"); err != nil {
		t.Fatalf("RevokeLink: %v", err)
	}
	assertRequest(t, seen.only(t), http.MethodPost, "/v1/integration/links/revoke", `{"link_token":"omx_link_abc"}`)
}

func TestPathParametersAreEncoded(t *testing.T) {
	const id = "a/b c?d#e"
	const encoded = "a%2Fb%20c%3Fd%23e"
	cases := []struct {
		name string
		call func(*Client) (*Usage, error)
		uri  string
	}{
		{"usage", func(c *Client) (*Usage, error) { return c.Usage(context.Background(), id) }, "/v1/integration/usages/" + encoded},
		{"confirm", func(c *Client) (*Usage, error) { return c.ConfirmUsage(context.Background(), id) }, "/v1/integration/usages/" + encoded + "/confirm"},
		{"void", func(c *Client) (*Usage, error) { return c.VoidUsage(context.Background(), id) }, "/v1/integration/usages/" + encoded + "/void"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, seen := stub(t, http.StatusOK, usagePending)
			if _, err := tc.call(client); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got := seen.only(t).URI; got != tc.uri {
				t.Errorf("uri = %q, want %q", got, tc.uri)
			}
		})
	}
}

func TestBaseURLTrailingSlashDoesNotDoubleUp(t *testing.T) {
	seen := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.add(exchange{URI: r.RequestURI})
		_, _ = io.WriteString(w, `{"linked":false,"active":false}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(testKey, WithBaseURL(server.URL+"/v1/"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.LinkStatus(context.Background(), "omx_link_abc"); err != nil {
		t.Fatalf("LinkStatus: %v", err)
	}
	if got := seen.only(t).URI; got != "/v1/integration/links/status" {
		t.Errorf("uri = %q", got)
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestInjectedHTTPClientIsUsed(t *testing.T) {
	var host string
	transport := roundTripper(func(r *http.Request) (*http.Response, error) {
		host = r.URL.Scheme + "://" + r.URL.Host + r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(`{"linked":true,"active":true}`)),
			Request:    r,
		}, nil
	})
	client, err := NewClient(testKey, WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := client.LinkStatus(context.Background(), "omx_link_abc")
	if err != nil {
		t.Fatalf("LinkStatus: %v", err)
	}
	if host != "https://api.onemax.az/v1/integration/links/status" {
		t.Errorf("address = %q", host)
	}
	if !got.Linked || !got.Active {
		t.Errorf("LinkStatus = %+v", got)
	}
}

func TestConnectionFailureIsTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	address := server.URL
	server.Close()
	client, err := NewClient(testKey, WithBaseURL(address))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Context(context.Background())
	var transportError *TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("err = %v, want *TransportError", err)
	}
	if transportError.Unwrap() == nil {
		t.Errorf("cause is nil")
	}
	var apiError *APIError
	if errors.As(err, &apiError) {
		t.Errorf("err also matches *APIError")
	}
}

func TestTimeoutIsTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(testKey, WithBaseURL(server.URL), WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	started := time.Now()
	_, err = client.Context(context.Background())
	var transportError *TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("err = %v, want *TransportError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Errorf("timeout took %v", elapsed)
	}
}

func TestCancelledContextIsTransportError(t *testing.T) {
	client, _ := stub(t, http.StatusOK, verifiedYes)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.VerifyCode(ctx, "123456")
	var transportError *TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("err = %v, want *TransportError", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to wrap context.Canceled", err)
	}
}

func TestNonJSONSuccessBodyIsTransportError(t *testing.T) {
	calls := map[string]func(*Client) error{
		"context": func(c *Client) error { _, err := c.Context(context.Background()); return err },
		"verify":  func(c *Client) error { _, err := c.VerifyCode(context.Background(), "123456"); return err },
		"usage":   func(c *Client) error { _, err := c.Usage(context.Background(), "u1"); return err },
		"revoke":  func(c *Client) error { return c.RevokeLink(context.Background(), "omx_link_abc") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			client, _ := stub(t, http.StatusOK, "<html>gateway</html>")
			err := call(client)
			var transportError *TransportError
			if !errors.As(err, &transportError) {
				t.Fatalf("err = %v, want *TransportError", err)
			}
		})
	}
}

func TestMalformedDatetimeIsTransportError(t *testing.T) {
	client, _ := stub(t, http.StatusOK, `{"usage_id":"u1","status":"pending","reference":null,"created_at":"yesterday","confirmed_at":null,"voided_at":null}`)
	_, err := client.Usage(context.Background(), "u1")
	var transportError *TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("err = %v, want *TransportError", err)
	}
}

func authorizeParams() AuthorizeParams {
	return AuthorizeParams{
		ClientID:      "omx_client_abc",
		RedirectURI:   "https://partner.example/callback?from=one max&x=1",
		CodeChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
	}
}

func TestAuthorizeURLLocales(t *testing.T) {
	cases := map[string]string{
		"":   "/baglanti",
		"az": "/baglanti",
		"ru": "/ru/baglanti",
		"en": "/en/baglanti",
	}
	client, err := NewClient(testKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	for locale, path := range cases {
		t.Run("locale "+locale, func(t *testing.T) {
			params := authorizeParams()
			params.Locale = locale
			params.State = "st at&e"
			address, err := client.AuthorizeURL(params)
			if err != nil {
				t.Fatalf("AuthorizeURL: %v", err)
			}
			parsed, err := url.Parse(address)
			if err != nil {
				t.Fatalf("address does not parse: %v", err)
			}
			if parsed.Scheme != "https" || parsed.Host != "onemax.az" || parsed.Path != path {
				t.Errorf("address = %q, want path %q on https://onemax.az", address, path)
			}
			want := url.Values{
				"response_type":         {"code"},
				"client_id":             {params.ClientID},
				"redirect_uri":          {params.RedirectURI},
				"code_challenge":        {params.CodeChallenge},
				"code_challenge_method": {"S256"},
				"state":                 {params.State},
			}
			if got := parsed.Query(); !reflect.DeepEqual(got, want) {
				t.Errorf("query = %v, want %v", got, want)
			}
		})
	}
}

func TestAuthorizeURLEncodesRedirectAndKeepsOrder(t *testing.T) {
	client, err := NewClient(testKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	params := authorizeParams()
	params.State = "xyz"
	address, err := client.AuthorizeURL(params)
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	_, query, found := strings.Cut(address, "?")
	if !found {
		t.Fatalf("address has no query: %q", address)
	}
	var names []string
	for _, pair := range strings.Split(query, "&") {
		name, value, _ := strings.Cut(pair, "=")
		names = append(names, name)
		if strings.ContainsAny(value, " :/?#") {
			t.Errorf("parameter %s is not encoded: %q", name, value)
		}
	}
	want := []string{"response_type", "client_id", "redirect_uri", "code_challenge", "code_challenge_method", "state"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("parameter order = %v, want %v", names, want)
	}
}

func TestAuthorizeURLLeavesStateOut(t *testing.T) {
	client, err := NewClient(testKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	address, err := client.AuthorizeURL(authorizeParams())
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("address does not parse: %v", err)
	}
	if parsed.Query().Has("state") {
		t.Errorf("address carries state: %q", address)
	}
	if !strings.HasSuffix(address, "&code_challenge_method=S256") {
		t.Errorf("address = %q", address)
	}
}

func TestAuthorizeURLRefusesUnknownLocale(t *testing.T) {
	client, err := NewClient(testKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	for _, locale := range []string{"de", "RU", "en-GB"} {
		params := authorizeParams()
		params.Locale = locale
		address, err := client.AuthorizeURL(params)
		if !errors.Is(err, ErrUnsupportedLocale) {
			t.Errorf("locale %q: err = %v, want ErrUnsupportedLocale", locale, err)
		}
		if address != "" {
			t.Errorf("locale %q: address = %q, want empty", locale, address)
		}
	}
}

func TestAuthorizeURLUsesSiteURL(t *testing.T) {
	client, err := NewClient(testKey, WithSiteURL("http://localhost:3000/"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	params := authorizeParams()
	params.Locale = "en"
	address, err := client.AuthorizeURL(params)
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	if !strings.HasPrefix(address, "http://localhost:3000/en/baglanti?response_type=code&client_id=omx_client_abc&") {
		t.Errorf("address = %q", address)
	}
}
