package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func newTestContext() (*gin.Context, *http.Request) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, req
}

func TestSetupRequestHeader_TrackingHeaders(t *testing.T) {
	t.Parallel()
	c, _ := newTestContext()
	c.Set(common.RequestIdKey, "req-123")
	c.Set(string(constant.ContextKeyUserId), 42)
	c.Set(string(constant.ContextKeyUserName), "alice")

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "sk-test"},
	}
	header := http.Header{}
	a := &Adaptor{}
	if err := a.SetupRequestHeader(c, &header, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if got := header.Get(trackingHeaderRequestId); got != "req-123" {
		t.Errorf("expected %s=req-123, got %q", trackingHeaderRequestId, got)
	}
	if got := header.Get(trackingHeaderUserId); got != "42" {
		t.Errorf("expected %s=42, got %q", trackingHeaderUserId, got)
	}
	if got := header.Get(trackingHeaderUser); got != "alice" {
		t.Errorf("expected %s=alice, got %q", trackingHeaderUser, got)
	}
	if got := header.Get("Authorization"); got != "Bearer sk-test" {
		t.Errorf("expected default Authorization header, got %q", got)
	}
}

func TestSetupRequestHeader_SessionUsernameFallback(t *testing.T) {
	t.Parallel()
	c, _ := newTestContext()
	c.Set(common.RequestIdKey, "req-session")
	c.Set(string(constant.ContextKeyUserId), 7)
	// Some auth paths set "username" directly rather than through the typed context key.
	c.Set("username", "bob")

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "sk-session"},
	}
	header := http.Header{}
	a := &Adaptor{}
	if err := a.SetupRequestHeader(c, &header, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if got := header.Get(trackingHeaderUser); got != "bob" {
		t.Errorf("expected session username %s=bob, got %q", trackingHeaderUser, got)
	}
	if got := header.Get(trackingHeaderUserId); got != "7" {
		t.Errorf("expected %s=7, got %q", trackingHeaderUserId, got)
	}
}

func TestSetupRequestHeader_MissingUsernameDoesNotFail(t *testing.T) {
	t.Parallel()
	c, _ := newTestContext()
	c.Set(common.RequestIdKey, "req-nouser")
	c.Set(string(constant.ContextKeyUserId), 99)
	// Intentionally no username set; DB lookup is allowed to fail.

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "sk-nouser"},
	}
	header := http.Header{}
	a := &Adaptor{}
	if err := a.SetupRequestHeader(c, &header, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if got := header.Get(trackingHeaderRequestId); got != "req-nouser" {
		t.Errorf("expected %s=req-nouser, got %q", trackingHeaderRequestId, got)
	}
	if got := header.Get(trackingHeaderUserId); got != "99" {
		t.Errorf("expected %s=99, got %q", trackingHeaderUserId, got)
	}
	// Username may be empty; importantly, the request was not failed.
	if got := header.Get("Authorization"); got != "Bearer sk-nouser" {
		t.Errorf("expected default Authorization header, got %q", got)
	}
}

func TestSetupRequestHeader_AuthorizationOverrideRespected(t *testing.T) {
	t.Parallel()
	c, _ := newTestContext()
	c.Set(common.RequestIdKey, "req-override")
	c.Set(string(constant.ContextKeyUserId), 1)
	c.Set(string(constant.ContextKeyUserName), "carol")

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "sk-ignored",
			HeadersOverride: map[string]interface{}{
				"Authorization":         "Bearer custom",
				trackingHeaderRequestId: "overridden-req",
				trackingHeaderUser:      "overridden-user",
				trackingHeaderUserId:    "overridden-id",
			},
		},
	}
	header := http.Header{}
	a := &Adaptor{}
	if err := a.SetupRequestHeader(c, &header, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	// Tracking headers are set by SetupRequestHeader; channel header overrides are
	// applied later by the common request helpers, so the adaptor-level Authorization
	// default is still skipped when an override is configured.
	if got := header.Get("Authorization"); got != "" {
		t.Errorf("expected Authorization to remain unset when override present, got %q", got)
	}
	if got := header.Get(trackingHeaderRequestId); got != "req-override" {
		t.Errorf("expected tracking header %s to be set before override merging, got %q", trackingHeaderRequestId, got)
	}
	if got := header.Get(trackingHeaderUser); got != "carol" {
		t.Errorf("expected tracking header %s to be set before override merging, got %q", trackingHeaderUser, got)
	}
	if got := header.Get(trackingHeaderUserId); got != "1" {
		t.Errorf("expected tracking header %s to be set before override merging, got %q", trackingHeaderUserId, got)
	}
}
