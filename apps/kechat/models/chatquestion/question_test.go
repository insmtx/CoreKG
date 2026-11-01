package chatquestion

import (
	"context"
	"fmt"
	"testing"

	"github.com/insmtx/corekg/apps/kechat/models/chatsession"
	"github.com/insmtx/corekg/apps/kechat/models/chattype"
	"github.com/insmtx/corekg/pkgs/types"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	dbtools "github.com/ygpkg/yg-go/dbtools/v2"
	_ "github.com/ygpkg/yg-go/dbtools/v2/mysqldrv"
	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"
)

// TestCreateQuestion 创建
func TestCreateQuestion(t *testing.T) {
	CreateQuestion(context.Background(), &chattype.ChatQuestion{
		Source: &chattype.Question{
			ReqID:    "2222222222222222222",
			Question: "222222222222222222222222222",
		},
	})
}

func TestDeleteQuestion(t *testing.T) {
	DeleteQuestion(context.Background(), "viy7vJgBxCIojy9uUZ1n")
}

func TestUpdateQuestion(t *testing.T) {
	UpdateQuestion(context.Background(), &chattype.ChatQuestion{
		ID: "vyy8vJgBxCIojy9u1Z11",
		Source: &chattype.Question{
			ReqID:    "22222222222222222221111111111111111",
			Question: "222222222222222222222222222asadasda",
		},
	})
}

func TestGetLLmSubQuestion(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=True&loc=Local",
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	str, _ := GetLLmSubQuestion(context.Background(), "我现在需要一个室外可POE供电的AP，请给我推荐下型号，并告诉我怎么配置", `根据检索到的文档信息，以下是关于室外POE供电AP的推荐及配置建议：

### 推荐型号
1. **UAP672X**  
   - **产品特点**：IP67防护等级，6.5KV防浪涌  
   - **适用场景**：室外环境  
   [Reference §1429[47]]

### 配置建议
1. **POE交换机选择**  
   - 需搭配支持POE供电的交换机，例如：  
     - **US218-HP**（整机最大输出功率240W）  
     - **US206-P**（4口POE）或 **US210-P**（8口POE），根据接入端数量选择  
     [Reference §1429[47], §1450[77,79,80,81]]  

2. **部署参数**  
   - **覆盖面积**：60平米/AP（吸顶AP参考值，室外需根据实际环境调整）  
   - **终端数量**：建议不超过70个终端/AP  
   [Reference §1450[77,79,80,81]]  

3. **安装注意事项**  
   - 确保交换机供电功率满足AP需求（如UAP672X需符合IP67防护等级对应的供电稳定性）  
   - 若需多AP部署，建议通过UR7103/UR7208网关管理带机量  
   [Reference §1450[77,79,80,81]]  

若需更详细的配置步骤（如具体交换机端口设置或AP固件配置），当前检索信息中未提供明确指导，建议补充相关技术文档以便进一步检索。  

> 注：以上推荐均基于文档中的设备清单及参数，实际部署需结合现场环境测试。
`)
	fmt.Println(str)
}

func TestListSessionQuestions(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	question, err := ListSessionQuestions(context.Background(), 0)
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
	for _, v := range question {
		logs.InfoContextf(ctx, "11111111111111111111111")
		logs.InfoContextf(ctx, "", v)
	}
}

func TestGetQuetionByID(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	question, err := GetQuetionByID(context.Background(), "wywwwJgBxCIojy9uxJ0W11")
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
	logs.InfoContextf(ctx, "", question)
}

func TestSearchAgentAllHistory(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	question, err := SearchAgentAllHistory(context.Background(), &StatisticsReq{
		AgentID: 732,
		// StartTime: time.Now().AddDate(0, 0, -1),
		// EndTime: time.Now(),
	})
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
	logs.InfoContextf(ctx, "----------------", question.Hits.Total)
	logs.InfoContextf(ctx, "", len(question.Hits.Hits))
}

func TestGetAgentHistoryCount(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	question, err := GetAgentHistoryCount(context.Background(), &StatisticsReq{
		AgentID: 732,
		// StartTime: time.Now().AddDate(0, 0, -1),
		// EndTime: time.Now(),
	})
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
	logs.InfoContextf(ctx, "----------------", question)
	// logs.InfoContextf(ctx,len(question.Hits.Hits))
}

func TestDeleteSessionQuestions(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	err := DeleteSessionQuestions(context.Background(), 169, 0)
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
}

func TestGetUnscopedQAByCompanyID(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	a, err := GetUnscopedQAByCompanyID(context.Background(), 2)
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
	println(a)
}

func TestDeleteForestSession(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat":    "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core":    "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"knownow": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	// forest 的所有session
	var sessions []*chattype.ChatSession
	err := dbutil.Chat().Model(&chattype.ChatSession{}).
		Where("id > 2502").
		Find(&sessions).Error
	if err != nil {
		logs.ErrorContext(ctx, err)
		return
	}
	for _, v := range sessions {
		err := DeleteSessionQuestions(ctx, v.Uin, v.ID)
		if err != nil {
			logs.ErrorContext(ctx, "RemoveChatSession error: %v", err)
			return
		}
		err = chatsession.DeleteSession(ctx, v.ID)
		if err != nil {
			logs.ErrorContext(ctx, "RemoveChatSession error: %v", err)
			return
		}
	}
}

func TestDeleteChatQuestion(t *testing.T) {
	dbtools.InitMultiDBConn(map[string]string{
		"chat":    "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"core":    "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
		"knownow": "mysql://yygu_test_rw:CHANGE_ME_PASSWORD@CHANGE_ME_HOST:63807/yygu_test?charset=utf8mb4&parseTime=true&loc=Local",
	})
	ctx := context.Background()
	InitHistoryESClient(ctx)
	// forest 的所有session
	var chatQuestions []*ChatQuestion
	err := dbutil.Chat().Model(&ChatQuestion{}).Find(&chatQuestions).Error
	if err != nil {
		logs.ErrorContextf(ctx, "chat_questions: %v", err)
		return
	}
	for _, v := range chatQuestions {
		logs.InfoContextf(ctx, "chat_questions: %v", v.ID)
		err := DeleteQuestion(ctx, fmt.Sprintf("chat_question_%d", v.ID))
		if err != nil {
			logs.ErrorContext(ctx, "RemoveChatSession error: %v", err)
		}
	}
}

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
