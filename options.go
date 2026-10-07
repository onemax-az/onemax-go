package onemax

import (
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://api.onemax.az/v1"
	DefaultSiteURL = "https://onemax.az"
	DefaultTimeout = 10 * time.Second
)

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithSiteURL(siteURL string) Option {
	return func(c *Client) {
		c.siteURL = siteURL
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

type verifyOptions struct {
	offerID string
}

type VerifyOption func(*verifyOptions)

func WithOffer(offerID string) VerifyOption {
	return func(o *verifyOptions) {
		o.offerID = offerID
	}
}

type confirmOptions struct {
	reference string
}

type ConfirmOption func(*confirmOptions)

func WithReference(reference string) ConfirmOption {
	return func(o *confirmOptions) {
		o.reference = reference
	}
}

type AuthorizeParams struct {
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	State         string
	Locale        string
}
