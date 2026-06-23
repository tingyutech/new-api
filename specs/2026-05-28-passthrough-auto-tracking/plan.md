# Plan

1. Update `relay/channel/proxy/adaptor.go`.
   - Import `strconv`, root `constant`, and `model`.
   - Add constants for `X-NewApi-Request-Id`, `X-NewApi-User`, and `X-NewApi-User-Id`.
   - Add a small helper that reads request id, user id, and username from the Gin context and sets non-empty tracking headers on the outbound `http.Header`.

2. Call the helper from `(*Adaptor).SetupRequestHeader`.
   - Keep the existing content negotiation setup from `channel.SetupApiRequestHeader`.
   - Keep the existing Authorization behavior.
   - Apply tracking headers before returning from `SetupRequestHeader` so all passthrough relay modes share the same behavior.

3. Add focused tests for passthrough header injection.
   - Cover context values containing request id, user id, and username.
   - Cover session-style username fallback by setting `"username"` directly on the Gin context.
   - Cover missing username behavior by verifying the request still includes request id and user id headers.
   - Verify Authorization still defaults to `Bearer <api key>`.
   - Verify channel header overrides remain able to replace tracking headers through existing common helper behavior.

4. Run Go verification.
   - Run `go test ./relay/channel/proxy ./relay/channel`.
   - Run broader relay tests when the narrow package tests pass.
