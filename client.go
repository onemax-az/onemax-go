package onemax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var localePrefixes = map[string]string{
	"":   "",
	"az": "",
	"ru": "/ru",
	"en": "/en",
}

type Client struct {
	apiKey     string
	baseURL    string
	siteURL    string
	timeout    time.Duration
	httpClient *http.Client
}

type verifyCodeBody struct {
	Code    string `json:"code"`
	OfferID string `json:"offer_id,omitempty"`
}

type confirmBody struct {
	Reference string `json:"reference,omitempty"`
}

type exchangeBody struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	RedirectURI  string `json:"redirect_uri"`
}

type linkBody struct {
	LinkToken string `json:"link_token"`
	OfferID   string `json:"offer_id,omitempty"`
}

type acknowledgement struct {
	OK bool `json:"ok"`
}

func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	client := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		siteURL:    DefaultSiteURL,
		timeout:    DefaultTimeout,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(client)
	}
	client.baseURL = strings.TrimRight(client.baseURL, "/")
	client.siteURL = strings.TrimRight(client.siteURL, "/")
	return client, nil
}

func (c *Client) Context(ctx context.Context) (*Context, error) {
	var result Context
	if err := c.do(ctx, http.MethodGet, "/integration/context", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) VerifyCode(ctx context.Context, code string, opts ...VerifyOption) (*Verification, error) {
	options := collectVerifyOptions(opts)
	body := verifyCodeBody{Code: code, OfferID: options.offerID}
	var result Verification
	if err := c.do(ctx, http.MethodPost, "/integration/members/verify", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Usage(ctx context.Context, usageID string) (*Usage, error) {
	return c.usage(ctx, http.MethodGet, usagePath(usageID), nil)
}

func (c *Client) ConfirmUsage(ctx context.Context, usageID string, opts ...ConfirmOption) (*Usage, error) {
	var options confirmOptions
	for _, opt := range opts {
		opt(&options)
	}
	body := confirmBody{Reference: options.reference}
	return c.usage(ctx, http.MethodPost, usagePath(usageID)+"/confirm", body)
}

func (c *Client) VoidUsage(ctx context.Context, usageID string) (*Usage, error) {
	return c.usage(ctx, http.MethodPost, usagePath(usageID)+"/void", nil)
}

func (c *Client) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*LinkToken, error) {
	body := exchangeBody{Code: code, CodeVerifier: codeVerifier, RedirectURI: redirectURI}
	var result LinkToken
	if err := c.do(ctx, http.MethodPost, "/integration/links/token", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) LinkStatus(ctx context.Context, linkToken string) (*LinkStatus, error) {
	var result LinkStatus
	if err := c.do(ctx, http.MethodPost, "/integration/links/status", linkBody{LinkToken: linkToken}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) VerifyLink(ctx context.Context, linkToken string, opts ...VerifyOption) (*Verification, error) {
	options := collectVerifyOptions(opts)
	body := linkBody{LinkToken: linkToken, OfferID: options.offerID}
	var result Verification
	if err := c.do(ctx, http.MethodPost, "/integration/links/verify", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) RevokeLink(ctx context.Context, linkToken string) error {
	var result acknowledgement
	return c.do(ctx, http.MethodPost, "/integration/links/revoke", linkBody{LinkToken: linkToken}, &result)
}

func (c *Client) AuthorizeURL(params AuthorizeParams) (string, error) {
	prefix, ok := localePrefixes[params.Locale]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedLocale, params.Locale)
	}
	var address strings.Builder
	address.WriteString(c.siteURL)
	address.WriteString(prefix)
	address.WriteString("/baglanti?response_type=code&client_id=")
	address.WriteString(url.QueryEscape(params.ClientID))
	address.WriteString("&redirect_uri=")
	address.WriteString(url.QueryEscape(params.RedirectURI))
	address.WriteString("&code_challenge=")
	address.WriteString(url.QueryEscape(params.CodeChallenge))
	address.WriteString("&code_challenge_method=S256")
	if params.State != "" {
		address.WriteString("&state=")
		address.WriteString(url.QueryEscape(params.State))
	}
	return address.String(), nil
}

func collectVerifyOptions(opts []VerifyOption) verifyOptions {
	var options verifyOptions
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

func usagePath(usageID string) string {
	return "/integration/usages/" + url.PathEscape(usageID)
}

func (c *Client) usage(ctx context.Context, method, path string, body any) (*Usage, error) {
	var result Usage
	if err := c.do(ctx, method, path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return &TransportError{Message: "encoding the request body failed", Err: err}
		}
		payload = bytes.NewReader(encoded)
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, payload)
	if err != nil {
		return &TransportError{Message: "building the request failed", Err: err}
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return &TransportError{Message: "the request failed", Err: err}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return &TransportError{Message: "reading the response failed", Err: err}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(response.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &TransportError{Message: "the response is not the expected JSON", Err: err}
	}
	return nil
}
