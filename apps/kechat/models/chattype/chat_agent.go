package chattype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/insmtx/corekg/pkgs/types"
	"gorm.io/gorm"
)

// ScopeType 范围类型
type ScopeType string

const (
	// ScopeTypeCompany 公司
	ScopeTypeCompany ScopeType = "company"
	// ScopeTypeUser 用户
	ScopeTypeUser ScopeType = "user"
)

// AgentType 机器人类型
type AgentType string

const (
	// AgentTypePrompt 指令机器人
	AgentTypePrompt AgentType = "prompt"
	// AgentTypeRolePlay 角色扮演机器人
	AgentTypeRolePlay AgentType = "role_play"
	// AgentTypeWorkflow 工作流
	AgentTypeWorkflow AgentType = "workflow"
)

// PublicScope 公开范围
type PublicScope string

const (
	// PublicScopePrivate 私有，仅创建者可见
	PublicScopePrivate PublicScope = "private"
	// PublicScopePublic 公开，所有人可见
	PublicScopePublic PublicScope = "public"
	// PublicScopeCompany 公司内可见
	PublicScopeCompany PublicScope = "company"
	// PublicScopeCustom 自定义范围
	PublicScopeCustom PublicScope = "custom"
)

// PublishStatus 定义发布状态枚举
type PublishStatus string

const (
	// StatusDraft 未发布（草稿）
	StatusDraft PublishStatus = "draft"
	// StatusPublished 已发布
	StatusPublished PublishStatus = "published"
	// StatusOffline 已下线
	StatusOffline PublishStatus = "offline"
)

type ExternalStatus string

const (
	ExternalStatusNormal  = "normal"
	ExternalStatusDisable = "disabled"
)

// ChatAgent 机器人
type ChatAgent struct {
	gorm.Model
	// Uin 创建人ID
	Uin uint `gorm:"column:uin;type:int;not null;default:0;index:uin;comment:创建人ID" json:"uin"`
	// CompanyID 公司ID
	CompanyID uint `gorm:"column:company_id;type:int;not null;default:0;index:company_id;comment:公司ID" json:"company_id"`
	// AvatarURL 机器人头像
	AvatarURL string `gorm:"column:avatar_url;type:varchar(256);comment:机器人头像" json:"avatar_url"`
	// Name 机器人名称
	Name string `gorm:"column:name;type:varchar(50);not null;default:'';index:name,unique;comment:机器人名称" json:"name"`
	// ShowName 机器人显示名称
	ShowName string `gorm:"column:show_name;type:varchar(64);not null;default:'';index:show_name;comment:机器人显示名称" json:"show_name"`
	// AgentType 机器人类型
	AgentType AgentType `gorm:"column:agent_type;type:varchar(32);not null;default:'';index:agent_type;comment:机器人类型" json:"agent_type"`
	// CreatedType 创建类型
	CreatedType string `gorm:"column:created_type;type:varchar(32);not null;default:'user';index:created_type;comment:创建类型" json:"created_type"`
	// ManagerIDs 管理员ID
	ManagerIDs types.UintArray `gorm:"column:manager_ids;type:varchar(256);comment:管理员ID" json:"manager_ids"`
	// PublicScope 公开范围
	PublicScope PublicScope `gorm:"column:public_scope;type:varchar(32);not null;default:'private';comment:公开范围" json:"public_scope"`
	// Version 版本
	Version uint `gorm:"column:version;type:int;not null;default:0;index:version;comment:版本" json:"version"`
	// Path 应用路径
	Path string `gorm:"column:path;type:varchar(256);not null;default:'';index:path;comment:应用路径" json:"path"`
	// PublishStatus 发布状态: draft-未发布, published-已发布, offline-已下线
	PublishStatus PublishStatus `gorm:"column:publish_status;type:varchar(32);default:'draft';index:publish_status;comment:发布状态" json:"publish_status"`

	ExternalStatus ExternalStatus `gorm:"column:external_status;type:varchar(63);comment:'外部调用状态';default:'disabled'" json:"external_status"`

	CozeSpaceID     string `gorm:"column:coze_space_id;type:varchar(64);not null;default:'';index:coze_space_id;comment:Coze空间ID" json:"coze_space_id"`
	CozeWorkflowID  string `gorm:"column:coze_workflow_id;type:varchar(64);not null;default:'';index:coze_workflow_id;comment:Coze工作流ID" json:"coze_workflow_id"`
	WorkflowVersion string `gorm:"column:workflow_version;type:varchar(64);not null;default:'';index:workflow_version;comment:Coze工作流版本" json:"workflow_version"`
}

type ChatAgentList []ChatAgent

// TableName 表名
func (ChatAgent) TableName() string {
	return TableNameAgent
}

type ChatAgentVersion struct {
	gorm.Model
	// AgentID 机器人ID
	AgentID uint `gorm:"column:agent_id;type:int;not null;default:0;index:agent_id;comment:机器人ID" json:"agent_id"`
	// Description 机器人描述
	Description string `gorm:"column:description;type:varchar(256);comment:描述" json:"description"`
	// ChatModelIDs 模型ID
	ChatModelIDs types.UintArray `gorm:"column:chat_model_ids;type:varchar(256);comment:模型ID" json:"chat_model_ids"`
	// Temperature 自定义温度
	Temperature float32 `gorm:"column:temperature;type:float;default:0.5;comment:自定义温度" json:"temperature"`
	// PromptTemplate 提示词模板
	PromptTemplate string `gorm:"column:prompt_template;type:text;not null;comment:提示词模板" json:"prompt_template"`
	// GreetingMessage 问候信息
	GreetingMessage string `gorm:"column:greeting_message;type:varchar(256);comment:问候信息" json:"greeting_message"`
	// Params 参数
	Params ParamsList `gorm:"column:params;type:text;comment:参数;serializer:json" json:"params"`
	// AgentType 机器人类型
	AgentType AgentType `gorm:"column:agent_type;type:varchar(32);not null;default:'';index:agent_type;comment:机器人类型" json:"agent_type"`
	// ForestOption 知识库ID
	ForestOption ForestChatOption `gorm:"column:forest_option;type:text;comment:知识库ID;serializer:json" json:"forest_option"`
}

func (ChatAgentVersion) TableName() string {
	return TableNameAgentVersion
}

// 输入参数类型
type InputType string

const (
	// 文本
	InputTypeText InputType = "text"
	// 下拉选框
	InputTypeSelect InputType = "select"
)

type Params struct {
	Input       string `json:"input"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsTitle     bool   `json:"is_title"`
	// 输入参数类型
	InputType InputType `json:"input_type"`
	// 下拉选项输入
	InputArray types.StringArray `json:"input_array"`
	// 是否必填
	IsRequired bool `json:"is_required"`
}

type ParamsList []Params

func (ep ParamsList) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *ParamsList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for ExamPosition")
	}
	return json.Unmarshal(bytes, ep)
}

// ForestChatOption 知识库文档相关配置
type ForestChatOption struct {
	PromptTemplate string `json:"prompt_template"`
	ForestIDs      []uint `json:"doc_forest_ids"`
}

func (ep ForestChatOption) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *ForestChatOption) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for ExamPosition")
	}
	return json.Unmarshal(bytes, ep)
}

// ChatAgentCollect 应用收藏
type ChatAgentCollect struct {
	gorm.Model
	// Uin 用户唯一标识
	Uin uint `gorm:"column:uin;type:bigint;not null;comment:用户唯一标识" json:"uin"`
	// AgentAppID 应用ID
	AgentAppID uint `gorm:"column:agent_app_id;type:bigint;not null;comment:应用ID" json:"agent_app_id"`
}

func (ChatAgentCollect) TableName() string {
	return TableNameAgentCollect
}
