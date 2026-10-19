# onemax-go

OneMax is a 1+1 membership club in Baku. Members pay for a plan and show a code at the counter of a partner venue to get the second item free. This library is the Go client for the OneMax partner API. It lets a partner's own system (a till, an ordering site, a mobile app backend) check a member and record a usage without the OneMax kassir screen.

## Install

```bash
go get github.com/onemax-az/onemax-go
```

The package name is `onemax`. It needs Go 1.22 or newer and uses only the standard library.

## Getting a key

1. OneMax switches API access on for the partner.
2. The owner creates a key in the partner panel under API.
3. The key belongs to one venue. Every call made with it acts for that venue.
4. Keep the key on the server. Never ship it in a browser or a mobile app.

Keys start with `omx_live_`.

## Quick start: a code

The member reads the six digit code from their OneMax card, or the partner scans the text of the QR code. `VerifyCode` answers yes or no. On yes it also opens a pending usage, which counts only once it is confirmed.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/onemax-az/onemax-go"
)

func main() {
	client, err := onemax.NewClient(os.Getenv("ONEMAX_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	venue, err := client.Context(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(venue.VenueName, venue.DailyLimit, len(venue.Offers))

	check, err := client.VerifyCode(ctx, "482913", onemax.WithOffer(venue.Offers[0].ID))
	if err != nil {
		log.Fatal(err)
	}
	if check.UsageID == nil {
		fmt.Println("no benefit:", check.Reason)
		return
	}

	usage, err := client.ConfirmUsage(ctx, *check.UsageID, onemax.WithReference("order-1042"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(usage.Status, usage.ConfirmedAt)
}
```

`WithOffer` is optional. Leave it out when the venue has a single offer.

Confirm the usage when the order is really given, and pass your own order id as the reference. Confirming again with the same reference is safe and returns the same usage, so a confirm that timed out can be sent again. If the order falls through, void the usage so it does not count against the member:

```go
usage, err := client.VoidUsage(ctx, usageID)
```

`client.Usage(ctx, usageID)` reads a usage back at any time.

## Account linking

A member can link their OneMax account to the partner's own app once. After that the partner checks them with a stored token and no code is needed.

Start on the partner's server. Create a PKCE pair, keep the verifier and the state in the member's session, and send the member to the permission page:

```go
pkce, err := onemax.GeneratePKCE()
if err != nil {
	return err
}

address, err := client.AuthorizeURL(onemax.AuthorizeParams{
	ClientID:      "omx_client_...",
	RedirectURI:   "https://partner.example/onemax/callback",
	CodeChallenge: pkce.Challenge,
	State:         state,
	Locale:        "en",
})
if err != nil {
	return err
}
http.Redirect(w, r, address, http.StatusFound)
```

`AuthorizeURL` makes no request. `State` and `Locale` are optional. The locale is `az`, `ru` or `en`, and anything else returns `onemax.ErrUnsupportedLocale`. `onemax.ChallengeFor(verifier)` derives the challenge for a verifier you already hold.

The member allows the link and comes back to the redirect address with `code` and `state`, or with `error=access_denied` if they refused. Compare the state with the one in the session, then exchange the code with the same verifier and the same redirect address:

```go
link, err := client.ExchangeCode(ctx, code, pkce.Verifier, "https://partner.example/onemax/callback")
if err != nil {
	return err
}
store(link.LinkToken)
```

Store the link token (it starts with `omx_link_`) beside your own user record. Later, check the member with it:

```go
status, err := client.LinkStatus(ctx, linkToken)
if err != nil {
	return err
}
fmt.Println(status.Linked, status.Active)

check, err := client.VerifyLink(ctx, linkToken, onemax.WithOffer(offerID))
if err != nil {
	return err
}
if check.UsageID == nil {
	fmt.Println("no benefit:", check.Reason)
	return nil
}
_, err = client.ConfirmUsage(ctx, *check.UsageID, onemax.WithReference(orderID))
return err
```

`LinkStatus` only answers yes or no. `VerifyLink` answers yes or no and, on yes, opens a pending usage that is confirmed or voided exactly as in the code flow.

A token whose link the member removed is not an error. `LinkStatus` answers `Linked: false` and `VerifyLink` answers `Reason: onemax.ReasonNotLinked`. To remove a link from the partner's side, call `client.RevokeLink(ctx, linkToken)`.

The partner never learns the member's name, email, phone or photo.

## Test mode

Build and try your integration before a real member is involved. Create a test key in the partner panel: it starts with `omx_test_` and works on the same address with the same methods. Nothing a test key does touches real members, usages or statistics. Going live means swapping the key.

```go
client, err := onemax.NewClient("omx_test_...")
if err != nil {
	log.Fatal(err)
}
fmt.Println(client.Mode())
```

`client.Mode()` is `onemax.ModeTest` for a test key and `onemax.ModeLive` otherwise. Every result also carries `Mode`, the mode the API answered in.

| Code | Test member | `Reason` |
| --- | --- | --- |
| `000001` | Active member, never runs out | `ok` |
| `000002` | One use left today, then `limit_reached` | `ok` |
| `000003` | Reached the limit | `limit_reached` |
| `000004` | No subscription | `not_subscribed` |
| `000005` | Offer not available now | `offer_unavailable` |

Any other code returns `invalid_code`. The partner panel's test mode tab also shows a renewing QR code for each test member, and lists your recent test requests.

For account linking, build the permission page address with the test client ID from the panel. The test page asks for no sign-in: you pick a test member and allow or deny, and it returns to your address as the live page does. Exchange the code with your test key.

## Results

`VerifyCode` and `VerifyLink` return a `*Verification`.

| Field | Type | Meaning |
| --- | --- | --- |
| `Active` | `bool` | The member has a subscription that works today. |
| `Usable` | `bool` | The member may use the offer right now. |
| `Reason` | `onemax.Reason` | Why, as one of the values below. |
| `UsageID` | `*string` | Set when `Usable` is true. Confirm or void it. |
| `DailyLimit` | `int` | How many usages a member has at this venue per day. |

| Reason | Constant | Meaning |
| --- | --- | --- |
| `ok` | `ReasonOK` | The member may use the offer. A pending usage was opened. |
| `not_subscribed` | `ReasonNotSubscribed` | The member has no working subscription. |
| `limit_reached` | `ReasonLimitReached` | The member used up today's limit at this venue. |
| `offer_unavailable` | `ReasonOfferUnavailable` | The offer is not open at this time. |
| `invalid_code` | `ReasonInvalidCode` | The code is wrong or has expired. |
| `not_linked` | `ReasonNotLinked` | The link behind the token was removed. |

`Reason` and `UsageStatus` are string types. A value this version does not know is kept as it arrived. A usage has the status `UsageStatusPending`, `UsageStatusConfirmed` or `UsageStatusVoided`, and its times are `time.Time` values, with `nil` for a time that has not happened.

## Errors

Every method returns one of two error types.

`*onemax.TransportError` means the request never produced a usable answer: the connection failed, the timeout passed, the context was cancelled, or the body was not the expected JSON. It wraps the cause, so `errors.Is(err, context.DeadlineExceeded)` works.

`*onemax.APIError` means the API answered with an error. It carries `Status`, `Code`, `Message`, `Details`, `RequestID` and `RetryAfter`. The message is Azerbaijani and meant for logs. Decide behaviour from the code. Match the kind of error with `errors.Is` and a sentinel:

| Sentinel | Status | Codes |
| --- | --- | --- |
| `ErrAuthentication` | 401 | `invalid_api_key` |
| `ErrAccessDisabled` | 403 | `api_access_disabled` |
| `ErrNotFound` | 404 | `usage_not_found` |
| `ErrConflict` | 409 | `usage_limit_reached`, `too_late`, `not_applicable`, `already_applied`, `reference_used`, `reference_mismatch` |
| `ErrInvalidRequest` | 400, 422 | `offer_required`, `unknown_offer`, `link_code_invalid`, `validation_error` |
| `ErrRateLimited` | 429 | `rate_limited` |
| `ErrServer` | 500 and above | any |

The match is by HTTP status, so a code added later on a known status still matches the right sentinel. Any other status is an `*APIError` that matches none of them. An error answer that is not the usual JSON has the code `unknown_error` and the first 200 characters of the body as its message. `RetryAfter` is the number of seconds to wait, set on a 429 when the API gives one.

```go
usage, err := client.ConfirmUsage(ctx, usageID, onemax.WithReference(orderID))

var apiErr *onemax.APIError
var transportErr *onemax.TransportError
switch {
case err == nil:
	fmt.Println("confirmed at", usage.ConfirmedAt)
case errors.Is(err, onemax.ErrRateLimited):
	if errors.As(err, &apiErr) && apiErr.RetryAfter != nil {
		time.Sleep(time.Duration(*apiErr.RetryAfter) * time.Second)
	}
case errors.Is(err, onemax.ErrConflict):
	errors.As(err, &apiErr)
	log.Printf("cannot confirm: %s (request %s)", apiErr.Code, apiErr.RequestID)
case errors.As(err, &transportErr):
	log.Printf("no answer, safe to send again with the same reference: %v", err)
default:
	log.Printf("onemax: %v", err)
}
```

## Limits

- The member's daily limit applies exactly as it does at a counter.
- Each key may make 120 requests a minute. Beyond that the API answers 429.
- The library never retries by itself. `ConfirmUsage` with a reference is safe to send again.
- The default timeout is 10 seconds. Change it with `onemax.WithTimeout`, and pass a context with a deadline for a single call.

## Configuration

```go
client, err := onemax.NewClient(
	apiKey,
	onemax.WithBaseURL("https://api.onemax.az/v1"),
	onemax.WithSiteURL("https://onemax.az"),
	onemax.WithTimeout(5*time.Second),
	onemax.WithHTTPClient(httpClient),
)
```

All options are optional and the values above, apart from the timeout, are the defaults. `NewClient` returns `onemax.ErrMissingAPIKey` for an empty key. A `Client` is safe to share between goroutines.

## Development

```bash
gofmt -l .
go vet ./...
go test -race ./...
```

`gofmt -l .` must print nothing. The tests run against a local stub server and never touch the network. The race detector needs cgo, so on a machine without a C compiler run `go test ./...` instead.

## Releasing

Set the version in `version.go`, add a `## x.y.z` section to `CHANGELOG.md`, commit, then push a tag
`vx.y.z`. The `Publish` workflow checks that the tag equals the package version, runs the checks,
asks the Go module proxy to index the version and creates the GitHub release from the changelog section. A tag that does not match the
version publishes nothing.

## License

MIT. See [LICENSE](LICENSE).
