package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gclm/octopus/internal/server/middleware"
	"github.com/gclm/octopus/internal/server/resp"
	"github.com/gclm/octopus/internal/server/router"
	"github.com/gin-gonic/gin"
)

func copilotOAuthConfig() (clientID, scope, deviceCodeURL, accessTokenURL string) {
	clientID = strings.TrimSpace(os.Getenv("OCTOPUS_COPILOT_CLIENT_ID"))
	if clientID == "" {
		clientID = "151ef1b1b0345b2351ca"
	}

	scope = strings.TrimSpace(os.Getenv("OCTOPUS_COPILOT_SCOPE"))
	if scope == "" {
		scope = "copilot"
	}

	deviceCodeURL = strings.TrimSpace(os.Getenv("OCTOPUS_COPILOT_DEVICE_CODE_URL"))
	if deviceCodeURL == "" {
		deviceCodeURL = "https://github.com/login/device/code"
	}

	accessTokenURL = strings.TrimSpace(os.Getenv("OCTOPUS_COPILOT_ACCESS_TOKEN_URL"))
	if accessTokenURL == "" {
		accessTokenURL = "https://github.com/login/oauth/access_token"
	}

	return
}

type copilotDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type copilotPollRequest struct {
	DeviceCode string `json:"device_code"`
}

type copilotPollResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Error       string `json:"error,omitempty"`
}

func copilotRequestDeviceCode(c *gin.Context) {
	clientID, scope, deviceCodeURL, _ := copilotOAuthConfig()
	body := fmt.Sprintf(`{"client_id":"%s","scope":"%s"}`, clientID, scope)
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		deviceCodeURL, strings.NewReader(body))
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	defer httpResp.Body.Close()

	var result copilotDeviceCodeResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		resp.Error(c, http.StatusBadGateway, "failed to decode GitHub response: "+err.Error())
		return
	}

	if result.DeviceCode == "" {
		resp.Error(c, http.StatusBadGateway, "empty device_code from GitHub")
		return
	}

	resp.Success(c, result)
}

func copilotPollToken(c *gin.Context) {
	var req copilotPollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if req.DeviceCode == "" {
		resp.Error(c, http.StatusBadRequest, "device_code is required")
		return
	}

	clientID, _, _, accessTokenURL := copilotOAuthConfig()
	body := fmt.Sprintf(
		`{"client_id":"%s","device_code":"%s","grant_type":"urn:ietf:params:oauth:grant-type:device_code"}`,
		clientID, req.DeviceCode,
	)
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		accessTokenURL, strings.NewReader(body))
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	defer httpResp.Body.Close()

	var result copilotPollResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		resp.Error(c, http.StatusBadGateway, "failed to decode GitHub response: "+err.Error())
		return
	}

	resp.Success(c, result)
}

func init() {
	router.NewGroupRouter("/api/v1/channel/copilot").
		Use(middleware.Auth()).
		AddRoute(
			router.NewRoute("/device-code", http.MethodPost).
				Handle(copilotRequestDeviceCode),
		)
	router.NewGroupRouter("/api/v1/channel/copilot").
		Use(middleware.Auth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/poll-token", http.MethodPost).
				Handle(copilotPollToken),
		)
}
