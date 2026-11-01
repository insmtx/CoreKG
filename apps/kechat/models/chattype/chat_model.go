package chattype

import (
	"time"

	"gorm.io/gorm"
)

type PublecType string

const (
	PublecTypeSystem PublecType = "system"
)

type SupportFunctionCall string

const (
	SupportFunctionCallSupported   SupportFunctionCall = "supported"
	SupportFunctionCallUnsupported SupportFunctionCall = "unsupported"
)

// ChatModel Chat模型表
type ChatModel struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	// ShowName 模型的显示名称
	ShowName string `gorm:"column:show_name;type:varchar(64);not null;comment:'模型的显示名称';index" json:"show_name"`
	// APIKey API密钥
	APIKey     string `gorm:"column:api_key;type:varchar(255);comment:'API密钥'" json:"api_key"`
	ModelGroup string `gorm:"column:model_group;type:varchar(64);comment:'模型分组'" json:"model_group"`
	// CozeModelID 对应 coze model_instance 表的 ID
	CozeModelID uint `gorm:"column:coze_model_id;type:bigint;not null;default:0;comment:'对应coze model_instance id';index" json:"coze_model_id"`
	// Priority 优先级，值越大优先级越高
	Priority uint `gorm:"column:priority;type:smallint;not null;default:0;comment:'优先级，值越大优先级越高'" json:"priority"`
	// ModelName 模型名称
	ModelName string `gorm:"column:model_name;type:varchar(64);not null;comment:'模型名称'" json:"model_name"`
	// ModelUrl 模型URL
	ModelUrl string `gorm:"column:model_url;type:varchar(255);comment:'模型URL'" json:"model_url"`
	// ModelType 公开类型
	PublecType PublecType `gorm:"column:public_type;type:varchar(32);comment:'公开类型'" json:"public_type"`
	// 模型供应商 baidu、aliyun、tencent、deepseak、openai
	ModelProvider string `gorm:"column:model_provider;type:varchar(50);comment:'模型供应商'" json:"model_provider"`
	// HeadURL 模型图片地址
	HeadURL             string              `gorm:"column:head_url;type:varchar(255);comment:'模型图片地址'" json:"head_url"`
	SupportFunctionCall SupportFunctionCall `gorm:"column:support_function_call;type:varchar(32);not null;default 'supported';comment:'是否支持 function call, supported/unsupported'" json:"support_function_call"`
}

type ChatModelList []ChatModel

// TableName 表名
func (ChatModel) TableName() string {
	return TableNameChatModel
}

// ChatModel Chat模型表
type ChatModelDTO struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	// ShowName 模型的显示名称
	ShowName string `gorm:"column:show_name;type:varchar(64);not null;comment:'模型的显示名称';index" json:"show_name"`
	// ModelName 模型名称
	ModelName string `gorm:"column:model_name;type:varchar(64);not null;comment:'模型名称'" json:"model_name"`
	// ModelType 公开类型
	PublecType PublecType `gorm:"column:public_type;type:varchar(32);comment:'公开类型'" json:"public_type"`
	// 模型供应商 baidu、aliyun、tencent、deepseak、openai
	ModelProvider string `gorm:"column:model_provider;type:varchar(50);comment:'模型供应商'" json:"model_provider"`
	// HeadURL 模型图片地址
	HeadURL  string `gorm:"column:head_url;type:varchar(255);comment:'模型图片地址'" json:"head_url"`
	UserName string `gorm:"column:user_name;type:varchar(64);not null;comment:'用户名称'" json:"user_name"`
	// ModelGroup 模型分组
	ModelGroup string `gorm:"column:model_group;type:varchar(64);comment:'模型分组'" json:"model_group"`
	// CozeModelID 对应 coze model_instance 表的 ID
	CozeModelID uint `gorm:"column:coze_model_id;type:bigint;not null;default:0;comment:'对应coze model_instance id';index" json:"coze_model_id"`
	// usage_count 使用次数
	UsageCount uint `gorm:"-" json:"usage_count"`
	// IsLastUsed 是否是最近使用的模型
	IsLastUsed bool `gorm:"-" json:"is_last_used"`
	// LastUsedAt 最近使用时间
	LastUsedAt *time.Time `gorm:"-" json:"last_used_at"`
}
