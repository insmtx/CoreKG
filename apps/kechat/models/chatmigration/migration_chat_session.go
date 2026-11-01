package chatmigration

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/insmtx/corekg/apps/kechat/models/chatmodel"
	"github.com/insmtx/corekg/apps/kechat/models/chatquestion"
	"github.com/insmtx/corekg/apps/kechat/models/chatsession"
	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/insmtx/corekg/apps/kecore/models/foresttype"
	"github.com/insmtx/corekg/pkgs/types"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"
)

type ChatQuestion struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	// ChatSessionID 群ID
	ChatSessionID uint `gorm:"column:chat_session_id;type:bigint;not null;default:0;comment:'群ID';index" json:"chat_session_id"`
	// APIKeyID APIKeyID
	APIKeyID uint `gorm:"column:api_key_id;type:bigint;not null;default:0;comment:'APIKeyID';index" json:"api_key_id"`

	// StreamKey 流密钥
	StreamKey string `gorm:"column:stream_key;type:varchar(32);comment:'流密钥';index" json:"stream_key,omitempty"`

	// RealIP  用户IP
	RealIP string `gorm:"column:real_ip;type:varchar(32);not null;default:'';comment:'用户IP'" json:"real_ip"`
	// From 来源
	From string `gorm:"column:from;type:varchar(32);not null;default:'';comment:'来源'" json:"from"`

	// 聊天信息
	// Question 问题
	Question string `gorm:"column:question;type:text;comment:'问题'" json:"question"`
	// 推理 reasoning
	Reasoning string `gorm:"column:reasoning;type:text;comment:'推理'" json:"reasoning"`
	// 推理耗时
	ReasoningSeconds int `gorm:"column:reasoning_seconds;type:integer;not null;default:0;comment:'推理耗时'" json:"reasoning_seconds"`
	// Answer 回答
	Answer string `gorm:"column:answer;type:text;comment:'回答'" json:"answer"`
	// CostSeconds 耗时
	CostSeconds int `gorm:"column:cost_seconds;type:integer;not null;default:0;comment:'耗时'" json:"cost_seconds"`
	// Status 状态
	Status chattype.QuestionStatus `gorm:"column:status;type:varchar(8);not null;default:'pending';comment:'状态'" json:"status"`
	// ModelName 模型名称
	ModelName string `gorm:"column:model_name;type:varchar(255);not null;index:model_name" json:"model_name"`
	// ModelID 模型ID
	ModelID uint `gorm:"column:model_id;type:int;not null;default:0;index:model_id;comment:模型ID" json:"model_id"`
	//BaseAgentID 机器人ID
	BaseAgentID uint `gorm:"column:base_agent_id;type:int;not null;default:0;index:base_agent_id;comment:机器人ID" json:"base_agent_id"`

	OutToken       int `gorm:"column:out_token;type:int;not null;default:0;comment:'输出token';index:out_token" json:"out_token"`
	CacheHitToken  int `gorm:"column:cache_hit_token;type:int;not null;default:0;comment:'缓存命中token';index:cache_hit_token" json:"cache_hit_token"`
	CacheMissToken int `gorm:"column:cache_miss_token;type:int;not null;default:0;comment:'缓存未命中token';index:cache_miss_token" json:"cache_miss_token"`
	// IsCharged 是否完成收费
	IsCharged bool `gorm:"type:smallint;not null" json:"is_charged"`

	// AnswerStars 用户对回答的打分
	AnswerStars int `gorm:"column:answer_stars;type:int;not null;default:0;comment:'回答星星';index:answer_stars" json:"answer_stars"`

	ImageUrlList       types.StringArray           `gorm:"column:image_url_list;type:text;comment:'图片列表'" json:"image_url_list"`
	QueryReferenceList chattype.QueryReferenceList `gorm:"column:query_reference_list;type:text;comment:'引用内容列表';serializer:json" json:"query_reference_list"`
	ChatReferenceList  chattype.QueryReferenceList `gorm:"column:chat_reference_list;type:text;comment:'引用内容列表';serializer:json" json:"chat_reference_list"`

	ExternalID string `gorm:"column:external_id;type:varchar(127);comment:'外部调用标识'" json:"external_id"`
}

func (ChatQuestion) TableName() string {
	return "chat_questions"
}

// MigrateChatQuestion 迁移chatagent的历史记录
func MigrateChatQuestion(ctx context.Context) error {
	var chatQuestions []*ChatQuestion
	err := dbutil.Chat().Model(&ChatQuestion{}).Find(&chatQuestions).Error
	if err != nil {
		logs.ErrorContextf(ctx, "chat_questions: %v", err)
		return err
	}
	for _, v := range chatQuestions {
		if err := chattype.ExistChatMigrate(ctx, v.ID, chattype.MigrateResourceTypeChatQuesion); err != nil {
			logs.WarnContextf(ctx, "chat_question_%d: %v", v.ID, err)
			continue
		}
		logs.InfoContextf(ctx, "chat_question_%d", v.ID)
		// 报存到es
		qu := &chattype.ChatQuestion{
			ID: fmt.Sprintf("chat_question_%d", v.ID),
			Source: &chattype.Question{
				Question:         v.Question,
				SessionID:        v.ChatSessionID,
				Uin:              v.Uin,
				CompanyID:        v.CompanyID,
				Answer:           v.Answer,
				ApiKeyID:         v.APIKeyID,
				Reasoning:        v.Reasoning,
				ReasoningSeconds: v.ReasoningSeconds,
				CostSeconds:      v.CostSeconds,
				Status:           v.Status,
				ModelID:          v.ModelID,
				BaseAgentID:      v.BaseAgentID,
				OutToken:         v.OutToken,
				CacheHitToken:    v.CacheHitToken,
				CacheMissToken:   v.CacheMissToken,
				ExternalID:       v.ExternalID,
				// ChatReferenceList:  &v.ChatReferenceList,
				QueryReferenceList: &v.QueryReferenceList,
				ImageUrlList:       v.ImageUrlList.Slice(),
				IsCharged:          v.IsCharged,
			},
		}
		err := chatquestion.CreateQuestionWithID(ctx, qu)
		if err != nil {
			logs.ErrorContextf(ctx, "chat_question_%d CreateQuestionWithID: %v", v.ID, err)
			return err
		}
		err = chattype.CreateChatMigrate(ctx, &chattype.ChatMigrate{
			ResourceType: chattype.MigrateResourceTypeChatQuesion,
			ResourceID:   v.ID,
			TargetIDStr:  qu.ID,
		})
		if err != nil {
			logs.ErrorContextf(ctx, "chat_question_%d CreateChatMigrate: %v", v.ID, err)
			return err
		}

	}
	return nil
}

// MigrateForestChat 迁移知识库问答历史记录
func MigrateForestChat(ctx context.Context) error {
	var forestSessions []*foresttype.KnownowQASession
	err := dbutil.Knownow().Model(&foresttype.KnownowQASession{}).
		Find(&forestSessions).Error
	if err != nil {
		// logs.Error(err)
		logs.ErrorContextf(ctx, "forest_sessions: %v", err)
		return err
	}
	for _, v := range forestSessions {
		logs.InfoContextf(ctx, "forest_session_%d", v.ID)
		mo := &chattype.ChatModel{}
		sess := &chattype.ChatSession{}
		mig, err := chattype.GetChatMigrate(ctx, v.ID, chattype.MigrateResourceTypeForestSession)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				if v.LLMModelID != 0 {
					mo, err = chatmodel.GetModelByID(ctx, v.LLMModelID)
					if err != nil {
						logs.ErrorContextf(ctx, "forest_session_%d: %v", v.ID, err)
						// return
						mo = &chattype.ChatModel{}
					}
				}
				sess = &chattype.ChatSession{
					Uin:              v.Uin,
					CompanyID:        v.CompanyID,
					Name:             v.Name,
					ModelName:        mo.ModelName,
					ModelID:          v.LLMModelID,
					ResourceType:     chattype.ResourceType(v.Type),
					BaseType:         chattype.ResourceQASessionBaseType(v.BaseType),
					FileID:           v.FileID,
					FileIDList:       v.FileIDList,
					ForestIDList:     v.ForestIDList,
					ExcelIDList:      v.ExcelIDList,
					ExcelSheetIDList: v.ExcelSheetIDList,
					EsIndex:          v.EsIndex,
				}
				err = chatsession.CreateSession(ctx, sess)
				if err != nil {
					// logs.Error(err)
					logs.ErrorContextf(ctx, "forest_session_%d: %v", v.ID, err)
					return err
				}
				err = chattype.CreateChatMigrate(ctx, &chattype.ChatMigrate{
					ResourceType: chattype.MigrateResourceTypeForestSession,
					ResourceID:   v.ID,
					TargetID:     sess.ID,
				})
				if err != nil {
					logs.ErrorContextf(ctx, "forest_session_%d CreateChatMigrate: %v", v.ID, err)
					return err
				}
			} else {
				logs.ErrorContextf(ctx, "forest_session_%d GetChatMigrate: %v", v.ID, err)
				return err
			}
		} else {
			sess, err = chatsession.GetChatSession(ctx, v.Uin, mig.TargetID)
			if err != nil {
				logs.ErrorContextf(ctx, "forest_session_%d GetChatSession: %v", v.ID, err)
				return err
			}
		}

		// 获取问答
		var forestQA []*foresttype.KnownowForestQA
		err = dbutil.Knownow().Table(foresttype.TableNameKnownowForestQA).
			Where("deleted_at is null").
			Where("uin = ?", sess.Uin).
			Where("session_id = ?", v.ID).Find(&forestQA).Error
		if err != nil {
			// logs.Error(err)
			logs.ErrorContextf(ctx, "forest_session_%d: %v", v.ID, err)
			return err
		}
		for _, v := range forestQA {
			if err := chattype.ExistChatMigrate(ctx, v.ID, chattype.MigrateResourceTypeForestQuestion); err != nil {
				logs.WarnContextf(ctx, "forest_question_%d: %v", v.ID, err)
				continue
			}
			logs.InfoContextf(ctx, "forest_question_%d", v.ID)
			qu := &chattype.ChatQuestion{
				ID: fmt.Sprintf("forest_question_%d", v.ID),
				Source: &chattype.Question{
					SessionID:          sess.ID,
					Uin:                v.Uin,
					CompanyID:          v.CompanyID,
					Question:           v.Question,
					Answer:             v.Answer,
					Reasoning:          v.Reasoning,
					ImageUrlList:       v.ImageUrlList.Slice(),
					ImageContent:       v.ImageContent,
					Status:             chattype.QuestionStatus(v.Status),
					QueryReferenceList: (*chattype.QueryReferenceList)(unsafe.Pointer(&v.QueryReferenceList)),
					// ChatReferenceList:  (*chattype.ChatReferenceList)(unsafe.Pointer(&v.ChatReferenceList)),
				},
			}
			err := chatquestion.CreateQuestionWithID(ctx, qu)
			if err != nil {
				// logs.Error(err)
				logs.ErrorContextf(ctx, "forest_question_%d CreateQuestionWithID: %v", v.ID, err)
				return err
			}
			err = chattype.CreateChatMigrate(ctx, &chattype.ChatMigrate{
				ResourceType: chattype.MigrateResourceTypeForestQuestion,
				ResourceID:   v.ID,
				TargetIDStr:  qu.ID,
			})
			if err != nil {
				logs.ErrorContextf(ctx, "forest_question_%d CreateChatMigrate: %v", v.ID, err)
				return err
			}
		}
	}
	return nil
}
