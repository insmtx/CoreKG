package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/account/models/apikey"
	"github.com/insmtx/corekg/apps/account/models/company"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/errcode"
	"github.com/ygpkg/yg-go/apis/runtime"
)

type WhoAmIResponse struct {
	apiobj.BaseResponse
	Response struct {
		Uin           uint   `json:"uin"`
		CompanyID     uint   `json:"company_id"`
		CompanyName   string `json:"company_name,omitempty"`
		APIKeyID      uint   `json:"api_key_id"`
		APIKeyPurpose string `json:"api_key_purpose,omitempty"`
	} `json:"response"`
}

func WhoAmI(ctx *gin.Context, _ *apiobj.BaseRequest, resp *WhoAmIResponse) {
	resp.Response.Uin = runtime.Uin(ctx)
	resp.Response.CompanyID = runtime.CompanyID(ctx)
	resp.Response.APIKeyID = runtime.APIKeyID(ctx)

	if resp.Response.APIKeyID > 0 {
		keyInfo, err := apikey.GetApiKeyByID(ctx, resp.Response.APIKeyID)
		if err != nil {
			resp.Code = errcode.ErrCode_InternalError
			resp.Message = "keapi_whoami_api_key_not_found"
			return
		}
		resp.Response.APIKeyPurpose = keyInfo.Purpose
	}

	if resp.Response.CompanyID == 0 {
		return
	}
	companyInfo, err := company.GetCompany(resp.Response.CompanyID)
	if err != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "keapi_whoami_company_not_found"
		return
	}
	resp.Response.CompanyName = companyInfo.Name
}
