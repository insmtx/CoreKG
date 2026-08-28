package apis

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/account/services/svccliauth"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/errcode"
)

type CLIAuthStartRequest struct {
	apiobj.BaseRequest
	Request struct {
		ClientName string `json:"client_name"`
		CLIVersion string `json:"cli_version"`
	}
}

type CLIAuthStartResponse struct {
	apiobj.BaseResponse
	Response struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
}

type CLIAuthPollRequest struct {
	apiobj.BaseRequest
	Request struct {
		DeviceCode string `json:"device_code"`
	}
}

type CLIAuthPollResponse struct {
	apiobj.BaseResponse
	Response struct {
		Status        string `json:"status"`
		APIKey        string `json:"api_key,omitempty"`
		APIKeyID      uint   `json:"api_key_id,omitempty"`
		APIKeyPurpose string `json:"api_key_purpose,omitempty"`
		UIN           uint   `json:"uin,omitempty"`
		CompanyID     uint   `json:"company_id,omitempty"`
		CompanyName   string `json:"company_name,omitempty"`
	}
}

type CLIAuthInfoRequest struct {
	apiobj.BaseRequest
	Request struct {
		UserCode string `json:"user_code"`
	}
}

type CLIAuthInfoResponse struct {
	apiobj.BaseResponse
	Response struct {
		ClientName string `json:"client_name,omitempty"`
		CLIVersion string `json:"cli_version,omitempty"`
		Status     string `json:"status"`
		ExpiresAt  int64  `json:"expires_at"`
	}
}

func CLIAuthStart(ctx *gin.Context, req *CLIAuthStartRequest, resp *CLIAuthStartResponse) {
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.Split(forwarded, ",")[0]
	}
	verificationURI := scheme + "://" + ctx.Request.Host + "/cli/authorize"
	result, err := svccliauth.Start(ctx, req.Request.ClientName, req.Request.CLIVersion, verificationURI)
	if err != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "keapi_cli_auth_start_failed"
		return
	}
	resp.Response.DeviceCode = result.DeviceCode
	resp.Response.UserCode = result.UserCode
	resp.Response.VerificationURI = result.VerificationURI
	resp.Response.ExpiresIn = result.ExpiresIn
	resp.Response.Interval = result.Interval
}

func CLIAuthPoll(ctx *gin.Context, req *CLIAuthPollRequest, resp *CLIAuthPollResponse) {
	if strings.TrimSpace(req.Request.DeviceCode) == "" {
		resp.Code = http.StatusBadRequest
		resp.Message = "keapi_cli_auth_device_code_required"
		return
	}
	result, err := svccliauth.Poll(ctx, req.Request.DeviceCode)
	if err != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "keapi_cli_auth_poll_failed"
		return
	}
	resp.Response.Status = result.Status
	resp.Response.APIKey = result.APIKey
	resp.Response.APIKeyID = result.APIKeyID
	resp.Response.APIKeyPurpose = result.APIKeyPurpose
	resp.Response.UIN = result.UIN
	resp.Response.CompanyID = result.CompanyID
	resp.Response.CompanyName = result.CompanyName
}

func CLIAuthInfo(ctx *gin.Context, req *CLIAuthInfoRequest, resp *CLIAuthInfoResponse) {
	if strings.TrimSpace(req.Request.UserCode) == "" {
		resp.Code = http.StatusBadRequest
		resp.Message = "keapi_cli_auth_user_code_required"
		return
	}
	session, err := svccliauth.GetByUserCode(ctx, req.Request.UserCode)
	if err != nil {
		resp.Code = http.StatusNotFound
		resp.Message = "keapi_cli_auth_session_not_found"
		return
	}
	resp.Response.ClientName = session.ClientName
	resp.Response.CLIVersion = session.CLIVersion
	resp.Response.Status = session.Status
	resp.Response.ExpiresAt = session.ExpiresAtUnix
}
