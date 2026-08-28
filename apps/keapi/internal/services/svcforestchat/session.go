package svcforestchat

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/insmtx/corekg/apps/keapi/internal/dto/dtokeapi"
	"github.com/insmtx/corekg/apps/kechat/models/chatmodel"
	"github.com/insmtx/corekg/apps/kechat/models/chatquestion"
	"github.com/insmtx/corekg/apps/kechat/models/chatsession"
	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/ygpkg/yg-go/apis/runtime"
	"gorm.io/gorm"
)

var ErrChatSessionNotFound = errors.New("chat session not found")

func CreateChatSession(ctx *gin.Context, req *dtokeapi.CreateChatSessionRequest) (*dtokeapi.ChatSessionItem, error) {
	model, err := selectChatModel(ctx)
	if err != nil {
		return nil, err
	}

	var session *chattype.ChatSession
	if req.Request.ForestID > 0 {
		session, err = buildForestSession(ctx, model, req.Request.ForestID, req.Request.Name)
	} else {
		session, err = buildForestFileSession(ctx, model, req.Request.ForestFileIDs, req.Request.Name)
	}
	if err != nil {
		return nil, err
	}
	if err := chatsession.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return NewChatSessionItem(session), nil
}

func BatchGetChatSession(ctx *gin.Context, sessionIDs []uint) ([]*dtokeapi.ChatSessionItem, error) {
	sessions := make([]*chattype.ChatSession, 0, len(sessionIDs))
	if err := dbutil.Chat().WithContext(ctx).
		Model(&chattype.ChatSession{}).
		Where("id IN ?", sessionIDs).
		Where("uin = ?", runtime.Uin(ctx)).
		Where("company_id = ?", runtime.CompanyID(ctx)).
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	sessionMap := make(map[uint]*chattype.ChatSession, len(sessions))
	for _, session := range sessions {
		sessionMap[session.ID] = session
	}

	items := make([]*dtokeapi.ChatSessionItem, 0, len(sessions))
	seen := make(map[uint]struct{}, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if _, ok := seen[sessionID]; ok {
			continue
		}
		seen[sessionID] = struct{}{}
		session, ok := sessionMap[sessionID]
		if !ok {
			continue
		}
		items = append(items, NewChatSessionItem(session))
	}
	return items, nil
}

func getOwnedChatSession(ctx *gin.Context, sessionID uint) (*chattype.ChatSession, error) {
	session := &chattype.ChatSession{}
	if err := dbutil.Chat().WithContext(ctx).
		Model(&chattype.ChatSession{}).
		Where("id = ?", sessionID).
		Where("uin = ?", runtime.Uin(ctx)).
		Where("company_id = ?", runtime.CompanyID(ctx)).
		First(session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChatSessionNotFound
		}
		return nil, err
	}
	return session, nil
}

func getSessionModel(ctx *gin.Context, session *chattype.ChatSession) (*chattype.ChatModel, error) {
	if session == nil || session.ModelID == 0 {
		return selectChatModel(ctx)
	}
	model, err := chatmodel.GetModelByID(ctx, session.ModelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChatModelNotFound
		}
		return nil, err
	}
	return model, nil
}

func UpdateChatName(ctx *gin.Context, sessionID uint, name string) (*dtokeapi.ChatSessionItem, error) {
	session, err := getOwnedChatSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	session.Name = name
	if err := chatsession.UpdateChatSession(ctx, session); err != nil {
		return nil, err
	}
	return NewChatSessionItem(session), nil
}

func DeleteChatSession(ctx *gin.Context, sessionID uint) error {
	session, err := getOwnedChatSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := chatsession.DeleteSession(ctx, session.ID); err != nil {
		return err
	}
	return nil
}

func CreateChatMessage(ctx *gin.Context, sessionID uint, content string) (*dtokeapi.ChatSessionMessageItem, error) {
	session, err := getOwnedChatSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	questionEntity := &chattype.ChatQuestion{
		Source: &chattype.Question{
			ReqID:     runtime.RequestID(ctx),
			CompanyID: session.CompanyID,
			Uin:       session.Uin,
			ModelID:   session.ModelID,
			SessionID: session.ID,
			Question:  content,
			Status:    chattype.QuestionStatusPending,
		},
	}
	if err := chatquestion.CreateQuestion(ctx, questionEntity); err != nil {
		return nil, err
	}

	items := dtokeapi.NewChatSessionMessageItems(questionEntity, normalizeAnswer)
	if len(items) == 0 {
		return nil, ErrInvalidChatMessages
	}
	return items[0], nil
}

func ListChatSessionMessages(ctx *gin.Context, sessionID uint) ([]*dtokeapi.ChatSessionMessageItem, error) {
	questions, err := chatquestion.ListSessionQuestionsByUin(ctx, runtime.Uin(ctx), sessionID)
	if err != nil {
		return nil, err
	}

	items := make([]*dtokeapi.ChatSessionMessageItem, 0, len(questions)*2)
	for _, question := range questions {
		items = append(items, dtokeapi.NewChatSessionMessageItems(question, normalizeAnswer)...)
	}
	return items, nil
}

func NewChatSessionItem(session *chattype.ChatSession) *dtokeapi.ChatSessionItem {
	if session == nil {
		return nil
	}
	return &dtokeapi.ChatSessionItem{
		SessionID:     session.ID,
		Name:          session.Name,
		ForestFileIDs: session.FileIDList.Slice(),
		ForestIDs:     session.ForestIDList.Slice(),
		ModelName:     session.ModelName,
		CreatedAt:     session.CreatedAt,
	}
}
