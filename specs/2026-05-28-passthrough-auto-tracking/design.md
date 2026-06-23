# Design

## Scope

Add default outbound tracking headers only for the passthrough channel implementation (`relay/channel/proxy.Adaptor`). Other provider adaptors keep their current header behavior.

## Header Behavior

For every upstream request sent by the passthrough adaptor, add these headers:

- `X-NewApi-Request-Id`: current request id from `common.RequestIdKey`.
- `X-NewApi-User`: current request user's username.
- `X-NewApi-User-Id`: current request user's numeric user id as a string.

The headers are applied in the passthrough adaptor's `SetupRequestHeader` method so they cover the existing HTTP, multipart form, and WebSocket request paths that call `channel.DoApiRequest`, `channel.DoFormRequest`, and `channel.DoWssRequest`.

## Data Sources

Request id comes from `c.GetString(common.RequestIdKey)`. The request-id middleware already creates this value and also sends it to the client as `X-Oneapi-Request-Id`.

User id comes from `constant.ContextKeyUserId`, which maps to the existing Gin context key `"id"`.

Username comes from `constant.ContextKeyUserName`. For compatibility with session-auth paths that set `"username"` directly, the implementation also reads `c.GetString("username")`. If the username is still empty and the user id is available, it resolves the username through `model.GetUsernameById(userID, false)`.

## Override Order

The existing common request helpers apply channel header overrides after adaptor `SetupRequestHeader`. That behavior remains unchanged. A channel-level explicit header override can still replace these tracking headers if configured, because overrides are the existing highest-priority mechanism.

## Error Handling

Tracking header injection does not fail the relay request. If a username lookup fails, the request still proceeds with the request id and user id headers, and omits `X-NewApi-User`.

## Dependencies

No third-party libraries are introduced.
