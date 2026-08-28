package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/account/models/apikey"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/errcode"
	"github.com/ygpkg/yg-go/apis/runtime"
)

type RevokeCurrentAPIKeyResponse struct {
	apiobj.BaseResponse
	Response struct {
		Revoked bool `json:"revoked"`
	}
}

func RevokeCurrentAPIKey(ctx *gin.Context, _ *apiobj.BaseRequest, resp *RevokeCurrentAPIKeyResponse) {
	if runtime.APIKeyID(ctx) == 0 {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_api_key_required"
		return
	}
	if err := apikey.DeleteAPIKeyByID(ctx, runtime.APIKeyID(ctx)); err != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "keapi_api_key_revoke_failed"
		return
	}
	resp.Response.Revoked = true
}
