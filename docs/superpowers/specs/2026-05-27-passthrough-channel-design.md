# Passthrough Channel (Type 28) — Design Spec

## Overview

Add a new channel type (ID 28, "Passthrough") that forwards all requests to an upstream API without format conversion. Requests are proxied as-is with API key replacement, model mapping, and all standard channel infrastructure (retry, billing, rate limiting, header override, etc.).

Use case: connecting to upstream providers that already speak the same API format (OpenAI-compatible, Claude-compatible, etc.) without the gateway performing any format translation.

## Architecture

### Approach: Dedicated Proxy Adaptor

A new adaptor at `relay/channel/proxy/` implements the full `Adaptor` interface with passthrough semantics. This fits the existing layered architecture (Router → Controller → Helper → Adaptor) and requires no changes to the controller or helpers.

### Request Flow

```
Client → Router (determines RelayFormat from path)
       → Controller (parses request, billing, retry loop)
       → Helper (model mapping, then calls adaptor)
       → Proxy Adaptor:
           Convert*: returns input DTO unchanged
           GetRequestURL: baseURL + original path
           SetupRequestHeader: Bearer token auth
           DoRequest: standard HTTP execution
           DoResponse: delegates to existing format handlers
       → Upstream API
```

## Components

### 1. Constants & Registration

**`constant/channel.go`:**
- `ChannelTypePassthrough = 28`
- `ChannelBaseURLs[28]` = `""` (user-configured)
- `ChannelTypeNames[28]` = `"Passthrough"`

**`constant/api_type.go`:**
- `APITypePassthrough` — new iota value before `APITypeDummy`

**`common/api_type.go`:**
- `ChannelTypePassthrough → APITypePassthrough` mapping in `ChannelType2APIType`

**`relay/relay_adaptor.go`:**
- `case constant.APITypePassthrough: return &proxy.Adaptor{}`

### 2. Proxy Adaptor (`relay/channel/proxy/adaptor.go`)

```
type Adaptor struct{}
```

**Init**: No-op (no channel-specific state needed).

**GetRequestURL**: 
```
relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType)
```
Preserves the original request path. `/v1/messages` stays `/v1/messages`.

**SetupRequestHeader**:
- Calls `channel.SetupApiRequestHeader` for Content-Type/Accept forwarding
- Sets `Authorization: Bearer <key>` with header override check (same pattern as OpenAI adaptor)

**Convert methods** (all return input unchanged):
- `ConvertOpenAIRequest(c, info, request)` → `return request, nil`
- `ConvertClaudeRequest(c, info, request)` → `return request, nil`
- `ConvertGeminiRequest(c, info, request)` → `return request, nil`
- `ConvertEmbeddingRequest(c, info, request)` → `return request, nil`
- `ConvertImageRequest(c, info, request)` → `return request, nil`
- `ConvertAudioRequest(c, info, request)` → return the audio reader as-is
- `ConvertRerankRequest(c, relayMode, request)` → `return request, nil`
- `ConvertOpenAIResponsesRequest(c, info, request)` → `return request, nil`

Model mapping is already applied by `ModelMappedHelper` (which calls `request.SetModelName(info.UpstreamModelName)`) before these methods are called. No additional work needed.

**DoRequest**: 
- Uses `channel.DoApiRequest(a, c, info, requestBody)` for JSON requests
- Uses `channel.DoFormRequest(a, c, info, requestBody)` for audio/multipart requests
- All existing infrastructure (header override, proxy support, timeout, compression) works automatically

**DoResponse** — dispatches to existing response handlers based on format:

```
switch info.RelayFormat {
case RelayFormatClaude:
    → claude.ClaudeHandler / claude.ClaudeStreamHandler
    (respects RelayFormat: writes raw Claude data when format is Claude)

case RelayFormatGemini:
    Native Gemini handlers that parse for usage and write raw bytes back:
    → embedding URLs → gemini.NativeGeminiEmbeddingHandler
    → streaming → gemini.GeminiTextGenerationStreamHandler
    → non-streaming → gemini.GeminiTextGenerationHandler

case RelayFormatOpenAIResponses:
    → openai.OaiResponsesHandler / OaiResponsesStreamHandler

case RelayFormatOpenAIResponsesCompaction:
    → openai.OaiResponsesCompactionHandler

default (OpenAI and other formats):
    switch info.RelayMode {
    case AudioSpeech → openai.OpenaiTTSHandler
    case AudioTranscription/Translation → openai.OpenaiSTTHandler  
    case ImagesGenerations/Edits → openai.OpenaiHandlerWithUsage
    case Rerank → common_handler.RerankHandler
    default → openai.OpenaiHandler / OaiStreamHandler
    }
}
```

This gives accurate usage extraction for OpenAI, Claude, Gemini, and Responses formats using existing proven code. No new response-parsing logic.

**GetModelList**: Returns `openai.ModelList` — the OpenAI model list is generic enough to serve as a reasonable default.

**GetChannelName**: Returns `"Passthrough"`.

### 3. Frontend Changes

**`web/default/src/features/channels/constants.ts`:**
- Add `28: 'Passthrough'` to `CHANNEL_TYPES`
- Add `28` to `CHANNEL_TYPE_DISPLAY_ORDER`

**`web/default/src/features/channels/lib/channel-utils.ts`:**
- Add `28: 'OpenAI'` to `TYPE_TO_ICON` (generic icon)

**i18n locale files** (`web/default/src/i18n/locales/`):
- Add `"Passthrough": "Passthrough"` (en) and `"Passthrough": "透传"` (zh) etc.

## What Works Automatically (no new code)

| Feature | Mechanism |
|---------|-----------|
| API key replacement | `SetupRequestHeader` |
| Model mapping | `ModelMappedHelper` runs before adaptor |
| Header override/passthrough | `processHeaderOverride` in `DoApiRequest` |
| Param override | Applied by helpers before DoRequest |
| Retry logic | Controller retry loop |
| Channel auto-ban | `processChannelError` in controller |
| HTTP proxy support | `doRequest` checks `info.ChannelSetting.Proxy` |
| Multi-key | Standard channel infrastructure |
| Rate limiting | Middleware layer |
| Pre-billing/quota | Standard flow (token estimation) |
| Usage extraction | Existing format-specific response handlers |

## Limitations (by design)

- **No format conversion**: A Claude-format request goes to an upstream that speaks Claude. An OpenAI request goes to an OpenAI-compatible upstream. No cross-format translation.
- **No provider-specific features**: Thinking adapter, reasoning effort mapping, OpenRouter-specific headers, Azure deployment paths — none of these apply.
- **StreamOptions supported**: Added to `streamSupportedChannels` in `relay/common/relay_info.go` so streaming requests can include usage info.

## Files Changed

### New Files
- `relay/channel/proxy/adaptor.go` — Adaptor implementation (~80 lines)

### Modified Files
- `constant/channel.go` — Add `ChannelTypePassthrough`, update `ChannelBaseURLs`, `ChannelTypeNames`
- `constant/api_type.go` — Add `APITypePassthrough`
- `common/api_type.go` — Add mapping in `ChannelType2APIType`
- `relay/relay_adaptor.go` — Add case in `GetAdaptor`
- `relay/common/relay_info.go` — Add `ChannelTypePassthrough` to `streamSupportedChannels`
- `web/default/src/features/channels/constants.ts` — Add to `CHANNEL_TYPES` and display order
- `web/default/src/features/channels/lib/channel-utils.ts` — Add icon mapping
- `web/default/src/i18n/locales/*.json` — Add "Passthrough" translations
