package forestctl

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/keapi/internal/dto/dtokeapi"
	"github.com/insmtx/corekg/apps/keapi/internal/services/svcforestchat"
	"github.com/ygpkg/yg-go/apis/errcode"
)

// CreateChatSession 创建对话会话
func CreateChatSession(ctx *gin.Context, req *dtokeapi.CreateChatSessionRequest, resp *dtokeapi.CreateChatSessionResponse) {
	if !req.ValidCreateChatSession(&resp.BaseResponse) {
		return
	}

	item, err := svcforestchat.CreateChatSession(ctx, req)
	if err != nil {
		switch {
		case errors.Is(err, svcforestchat.ErrInvalidForestFiles):
			resp.Code = errcode.ErrCode_BadRequest
			resp.Message = "keapi_invalid_forest_file_ids"
		case errors.Is(err, svcforestchat.ErrInvalidForestScope):
			resp.Code = errcode.ErrCode_NotFound
			resp.Message = "keapi_forest_not_found"
		case errors.Is(err, svcforestchat.ErrChatModelNotFound):
			resp.Code = errcode.ErrCode_BadRequest
			resp.Message = "keapi_chat_model_not_found"
		default:
			resp.Code = errcode.ErrCode_InternalError
			resp.Message = "keapi_create_chat_session_failed"
		}
		return
	}
	resp.Response = *item
}

// BatchGetChatSession 批量查询对话会话信息
func BatchGetChatSession(ctx *gin.Context, req *dtokeapi.BatchGetChatSessionRequest, resp *dtokeapi.BatchGetChatSessionResponse) {
	if !req.ValidBatchGetChatSession(&resp.BaseResponse) {
		return
	}

	items, err := svcforestchat.BatchGetChatSession(ctx, req.Request.SessionIDs)
	if err != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "keapi_batch_get_chat_session_failed"
		return
	}
	resp.Response.Total = int64(len(items))
	resp.Response.Offset = 0
	resp.Response.Limit = len(req.Request.SessionIDs)
	resp.Response.Data = items
}

// UpdateChatName 更新对话会话名称
func UpdateChatName(ctx *gin.Context, req *dtokeapi.UpdateChatNameRequest, resp *dtokeapi.UpdateChatNameResponse) {
	if !req.ValidUpdateChatName(&resp.BaseResponse) {
		return
	}

	item, err := svcforestchat.UpdateChatName(ctx, req.Request.SessionID, req.Request.Name)
	if err != nil {
		switch {
		case errors.Is(err, svcforestchat.ErrChatSessionNotFound):
			resp.Code = errcode.ErrCode_NotFound
			resp.Message = "keapi_chat_session_not_found"
		default:
			resp.Code = errcode.ErrCode_InternalError
			resp.Message = "keapi_update_chat_name_failed"
		}
		return
	}
	resp.Response = *item
}

// DeleteChatSession 删除对话会话
func DeleteChatSession(ctx *gin.Context, req *dtokeapi.DeleteChatSessionRequest, resp *dtokeapi.DeleteChatSessionResponse) {
	if !req.ValidDeleteChatSession(&resp.BaseResponse) {
		return
	}

	if err := svcforestchat.DeleteChatSession(ctx, req.Request.SessionID); err != nil {
		switch {
		case errors.Is(err, svcforestchat.ErrChatSessionNotFound):
			resp.Code = errcode.ErrCode_NotFound
			resp.Message = "keapi_chat_session_not_found"
		default:
			resp.Code = errcode.ErrCode_InternalError
			resp.Message = "keapi_delete_chat_session_failed"
		}
		return
	}
	resp.Response = dtokeapi.DeleteChatSessionItem{
		SessionID: req.Request.SessionID,
		Deleted:   true,
	}
}

// CreateChatMessage 创建用户消息
func CreateChatMessage(ctx *gin.Context, req *dtokeapi.CreateChatMessageRequest, resp *dtokeapi.CreateChatMessageResponse) {
	if !req.ValidCreateChatMessage(&resp.BaseResponse) {
		return
	}

	item, err := svcforestchat.CreateChatMessage(ctx, req.Request.SessionID, req.Request.Content)
	if err != nil {
		switch {
		case errors.Is(err, svcforestchat.ErrChatSessionNotFound):
			resp.Code = errcode.ErrCode_NotFound
			resp.Message = "keapi_chat_session_not_found"
		case errors.Is(err, svcforestchat.ErrInvalidChatMessages):
			resp.Code = errcode.ErrCode_BadRequest
			resp.Message = "keapi_empty_message_content"
		default:
			resp.Code = errcode.ErrCode_InternalError
			resp.Message = "keapi_create_chat_message_failed"
		}
		return
	}
	resp.Response = *item
}

// ListChatSessionMessages 查询对话会话消息列表
func ListChatSessionMessages(ctx *gin.Context, req *dtokeapi.ListChatSessionMessagesRequest, resp *dtokeapi.ListChatSessionMessagesResponse) {
	if !req.ValidListChatSessionMessages(&resp.BaseResponse) {
		return
	}

	items, err := svcforestchat.ListChatSessionMessages(ctx, req.Request.SessionID)
	if err != nil {
		resp.Code = errcode.ErrCode_InternalError
		resp.Message = "keapi_list_chat_session_messages_failed"
		return
	}
	resp.Response.Data = items
	// TODO: support pagination/cursor and calculate has_more from the query result.
	resp.Response.HasMore = false
}
