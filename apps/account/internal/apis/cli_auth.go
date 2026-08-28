package apis

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/account/models/company"
	"github.com/insmtx/corekg/apps/account/services/svccliauth"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/errcode"
	"github.com/ygpkg/yg-go/apis/runtime"
)

type CLIAuthApprovalRequest struct {
	apiobj.BaseRequest
	Request struct {
		UserCode string `json:"user_code"`
	}
}

type CLIAuthApprovalResponse struct {
	apiobj.BaseResponse
	Response struct {
		Status string `json:"status"`
	}
}

func CLIAuthApprove(ctx *gin.Context, req *CLIAuthApprovalRequest, resp *CLIAuthApprovalResponse) {
	if strings.TrimSpace(req.Request.UserCode) == "" {
		resp.Code = http.StatusBadRequest
		resp.Message = "account_cli_auth_user_code_required"
		return
	}
	companyID := runtime.CompanyID(ctx)
	companyInfo, companyErr := company.GetCompany(companyID)
	if companyErr != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "account_cli_auth_company_not_found"
		return
	}
	if err := svccliauth.Approve(ctx, req.Request.UserCode, runtime.Uin(ctx), companyID, companyInfo.Name); err != nil {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "account_cli_auth_approve_failed"
		return
	}
	resp.Response.Status = "approved"
}

func CLIAuthDeny(ctx *gin.Context, req *CLIAuthApprovalRequest, resp *CLIAuthApprovalResponse) {
	if strings.TrimSpace(req.Request.UserCode) == "" {
		resp.Code = http.StatusBadRequest
		resp.Message = "account_cli_auth_user_code_required"
		return
	}
	if err := svccliauth.Deny(ctx, req.Request.UserCode); err != nil {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "account_cli_auth_deny_failed"
		return
	}
	resp.Response.Status = "denied"
}
