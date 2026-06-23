package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/common_handler"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	trackingHeaderRequestId = "X-NewApi-Request-Id"
	trackingHeaderUser      = "X-NewApi-User"
	trackingHeaderUserId    = "X-NewApi-User-Id"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	hasAuthOverride := false
	if len(info.HeadersOverride) > 0 {
		for k := range info.HeadersOverride {
			if strings.EqualFold(k, "Authorization") {
				hasAuthOverride = true
				break
			}
		}
	}
	if !hasAuthOverride {
		header.Set("Authorization", "Bearer "+info.ApiKey)
	}
	setupTrackingHeaders(c, header)
	return nil
}

func setupTrackingHeaders(c *gin.Context, header *http.Header) {
	if c == nil || header == nil {
		return
	}
	if requestId := c.GetString(common.RequestIdKey); requestId != "" {
		header.Set(trackingHeaderRequestId, requestId)
	}
	userIdVal, exists := c.Get(string(constant.ContextKeyUserId))
	userId := 0
	if exists {
		switch v := userIdVal.(type) {
		case int:
			userId = v
		case int64:
			userId = int(v)
		}
	}
	if userId > 0 {
		header.Set(trackingHeaderUserId, strconv.Itoa(userId))
	}
	username := c.GetString(string(constant.ContextKeyUserName))
	if username == "" {
		username = c.GetString("username")
	}
	if username == "" && userId > 0 {
		func() {
			defer func() {
				_ = recover()
			}()
			if resolved, err := model.GetUsernameById(userId, false); err == nil && resolved != "" {
				username = resolved
			}
		}()
	}
	if username != "" {
		header.Set(trackingHeaderUser, username)
	}
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode == relayconstant.RelayModeAudioSpeech {
		jsonData, err := common.Marshal(request)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(jsonData), nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	return common.ReaderOnly(storage), nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation {
		return channel.DoFormRequest(a, c, info, requestBody)
	} else if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		if info.IsStream {
			return claude.ClaudeStreamHandler(c, resp, info)
		}
		return claude.ClaudeHandler(c, resp, info)

	case types.RelayFormatGemini:
		if strings.Contains(info.RequestURLPath, ":embedContent") ||
			strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
			return gemini.NativeGeminiEmbeddingHandler(c, resp, info)
		}
		if info.IsStream {
			return gemini.GeminiTextGenerationStreamHandler(c, info, resp)
		}
		return gemini.GeminiTextGenerationHandler(c, info, resp)

	case types.RelayFormatOpenAIResponses:
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)

	case types.RelayFormatOpenAIResponsesCompaction:
		return openai.OaiResponsesCompactionHandler(c, resp)

	default:
		switch info.RelayMode {
		case relayconstant.RelayModeAudioSpeech:
			return openai.OpenaiTTSHandler(c, resp, info), nil
		case relayconstant.RelayModeAudioTranslation, relayconstant.RelayModeAudioTranscription:
			apiErr, usage := openai.OpenaiSTTHandler(c, resp, info, "")
			return usage, apiErr
		case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
			return openai.OpenaiHandlerWithUsage(c, info, resp)
		case relayconstant.RelayModeRerank:
			return common_handler.RerankHandler(c, info, resp)
		default:
			if info.IsStream {
				return openai.OaiStreamHandler(c, info, resp)
			}
			return openai.OpenaiHandler(c, info, resp)
		}
	}
}

func (a *Adaptor) GetModelList() []string {
	return openai.ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "Passthrough"
}
