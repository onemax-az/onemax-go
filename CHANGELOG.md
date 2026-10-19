# Changelog

## 0.2.0

- Test mode. A key that starts with `omx_test_` works against OneMax test members on the same address, so an integration can be built and tried before a real member is involved.
- `Client.Mode` says whether the client holds a live or a test key, as `ModeLive` or `ModeTest`.
- `Context`, `Verification`, `Usage`, `LinkToken` and `LinkStatus` carry `Mode`, the mode the API answered in. It is empty when talking to a server that does not send it.

## 0.1.0

First release.

- `Client` with `Context`, `VerifyCode`, `Usage`, `ConfirmUsage` and `VoidUsage` for checking a member by code and recording the usage.
- Account linking with `AuthorizeURL`, `ExchangeCode`, `LinkStatus`, `VerifyLink` and `RevokeLink`.
- `GeneratePKCE` and `ChallengeFor` for the PKCE verifier and challenge.
- `*APIError` with sentinel errors matched by HTTP status through `errors.Is`, and `*TransportError` for requests that produced no usable answer.
