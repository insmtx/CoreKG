package dtokeapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/ygpkg/yg-go/apis/apiobj"
	"github.com/ygpkg/yg-go/apis/errcode"
)

const chatSessionMessageIDLength = 32

// CreateChatSessionRequest 创建对话会话请求
type CreateChatSessionRequest struct {
	apiobj.BaseRequest
	Request struct {
		ForestFileIDs       []uint `json:"forest_file_ids"`
		LegacyForestFileIDs []uint `json:"forest_file_id,omitempty"`
		ForestID            uint   `json:"forest_id,omitempty"`
		Name                string `json:"name"`
	} `json:"request"`
}

func (req *CreateChatSessionRequest) ValidCreateChatSession(resp *apiobj.BaseResponse) bool {
	if req.Request.ForestID > 0 && (len(req.Request.ForestFileIDs) > 0 || len(req.Request.LegacyForestFileIDs) > 0) {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_chat_scope_conflict"
		return false
	}
	if len(req.Request.ForestFileIDs) == 0 && len(req.Request.LegacyForestFileIDs) > 0 {
		req.Request.ForestFileIDs = req.Request.LegacyForestFileIDs
	}
	return true
}

// BatchGetChatSessionRequest 批量查询对话会话请求
type BatchGetChatSessionRequest struct {
	apiobj.BaseRequest
	Request struct {
		SessionIDs []uint `json:"session_ids"`
	} `json:"request"`
}

func (req *BatchGetChatSessionRequest) ValidBatchGetChatSession(resp *apiobj.BaseResponse) bool {
	if len(req.Request.SessionIDs) == 0 {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_invalid_session_ids"
		return false
	}
	return true
}

type ChatSessionItem struct {
	SessionID     uint      `json:"session_id"`
	Name          string    `json:"name"`
	ForestFileIDs []uint    `json:"forest_file_id"`
	ForestIDs     []uint    `json:"forest_id"`
	ModelName     string    `json:"model_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateChatSessionResponse 创建对话会话响应
type CreateChatSessionResponse struct {
	apiobj.BaseResponse
	Response ChatSessionItem `json:"response"`
}

// BatchGetChatSessionResponse 批量查询对话会话响应
type BatchGetChatSessionResponse struct {
	apiobj.BaseResponse
	Response struct {
		apiobj.QueryResponse
		Data []*ChatSessionItem `json:"data"`
	} `json:"response"`
}

// UpdateChatNameRequest 更新对话会话名称请求
type UpdateChatNameRequest struct {
	apiobj.BaseRequest
	Request struct {
		SessionID uint   `json:"session_id"`
		Name      string `json:"name"`
	} `json:"request"`
}

func (req *UpdateChatNameRequest) ValidUpdateChatName(resp *apiobj.BaseResponse) bool {
	if req.Request.SessionID == 0 {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_invalid_session_id"
		return false
	}
	req.Request.Name = strings.TrimSpace(req.Request.Name)
	if req.Request.Name == "" {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_chat_name_empty"
		return false
	}
	return true
}

// UpdateChatNameResponse 更新对话会话名称响应
type UpdateChatNameResponse struct {
	apiobj.BaseResponse
	Response ChatSessionItem `json:"response"`
}

// DeleteChatSessionRequest 删除对话会话请求
type DeleteChatSessionRequest struct {
	apiobj.BaseRequest
	Request struct {
		SessionID uint `json:"session_id"`
	} `json:"request"`
}

func (req *DeleteChatSessionRequest) ValidDeleteChatSession(resp *apiobj.BaseResponse) bool {
	if req.Request.SessionID == 0 {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_invalid_session_id"
		return false
	}
	return true
}

type DeleteChatSessionItem struct {
	SessionID uint `json:"session_id"`
	Deleted   bool `json:"deleted"`
}

// DeleteChatSessionResponse 删除对话会话响应
type DeleteChatSessionResponse struct {
	apiobj.BaseResponse
	Response DeleteChatSessionItem `json:"response"`
}

// CreateChatMessageRequest 创建用户消息请求
type CreateChatMessageRequest struct {
	apiobj.BaseRequest
	Request struct {
		SessionID uint   `json:"session_id"`
		Content   string `json:"content"`
	} `json:"request"`
}

func (req *CreateChatMessageRequest) ValidCreateChatMessage(resp *apiobj.BaseResponse) bool {
	if req.Request.SessionID == 0 {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_invalid_session_id"
		return false
	}
	req.Request.Content = strings.TrimSpace(req.Request.Content)
	if req.Request.Content == "" {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_empty_message_content"
		return false
	}
	return true
}

// CreateChatMessageResponse 创建用户消息响应
type CreateChatMessageResponse struct {
	apiobj.BaseResponse
	Response ChatSessionMessageItem `json:"response"`
}

// ListChatSessionMessagesRequest 查询对话会话消息列表请求
type ListChatSessionMessagesRequest struct {
	apiobj.BaseRequest
	Request struct {
		SessionID uint `json:"session_id"`
	} `json:"request"`
}

func (req *ListChatSessionMessagesRequest) ValidListChatSessionMessages(resp *apiobj.BaseResponse) bool {
	if req.Request.SessionID == 0 {
		resp.Code = errcode.ErrCode_BadRequest
		resp.Message = "keapi_invalid_session_id"
		return false
	}
	return true
}

type ChatSessionMessageItem struct {
	MessageID string    `json:"message_id"`
	SessionID uint      `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ListChatSessionMessagesResponse 查询对话会话消息列表响应
type ListChatSessionMessagesResponse struct {
	apiobj.BaseResponse
	Response struct {
		Data    []*ChatSessionMessageItem `json:"data"`
		HasMore bool                      `json:"has_more"`
	} `json:"response"`
}

func NewChatSessionMessageItems(question *chattype.ChatQuestion, answerFilter func(string) string) []*ChatSessionMessageItem {
	if question == nil || question.Source == nil {
		return nil
	}
	source := question.Source
	items := make([]*ChatSessionMessageItem, 0, 2)
	if strings.TrimSpace(source.Question) != "" {
		items = append(items, &ChatSessionMessageItem{
			MessageID: BuildChatSessionMessageID(question, "user"),
			SessionID: source.SessionID,
			Role:      "user",
			Content:   source.Question,
			CreatedAt: source.CreatedAt,
		})
	}
	if strings.TrimSpace(source.Answer) != "" {
		answer := source.Answer
		if answerFilter != nil {
			answer = answerFilter(answer)
		}
		items = append(items, &ChatSessionMessageItem{
			MessageID: BuildChatSessionMessageID(question, "assistant"),
			SessionID: source.SessionID,
			Role:      "assistant",
			Content:   answer,
			CreatedAt: source.CreatedAt,
		})
	}
	return items
}

func BuildChatSessionMessageID(question *chattype.ChatQuestion, role string) string {
	parts := []string{"keapi_chat_session_message", role}
	if question == nil || question.Source == nil {
		return stableChatSessionMessageID(parts...)
	}

	source := question.Source
	questionID := strings.TrimSpace(question.ID)
	if questionID == "" {
		questionID = strconv.FormatInt(source.CreatedAt.UnixNano(), 10)
	}
	parts = append(parts, questionID, strconv.FormatUint(uint64(source.SessionID), 10))
	return stableChatSessionMessageID(parts...)
}

func stableChatSessionMessageID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		hash.Write([]byte(strconv.Itoa(len(part))))
		hash.Write([]byte(":"))
		hash.Write([]byte(part))
		hash.Write([]byte("|"))
	}
	messageID := hex.EncodeToString(hash.Sum(nil))
	if len(messageID) <= chatSessionMessageIDLength {
		return messageID
	}
	return messageID[:chatSessionMessageIDLength]
}
