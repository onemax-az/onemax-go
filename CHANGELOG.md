# Changelog

## 0.1.0

First release.

- `Client` with `Context`, `VerifyCode`, `Usage`, `ConfirmUsage` and `VoidUsage` for checking a member by code and recording the usage.
- Account linking with `AuthorizeURL`, `ExchangeCode`, `LinkStatus`, `VerifyLink` and `RevokeLink`.
- `GeneratePKCE` and `ChallengeFor` for the PKCE verifier and challenge.
- `*APIError` with sentinel errors matched by HTTP status through `errors.Is`, and `*TransportError` for requests that produced no usable answer.
