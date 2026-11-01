package chattype

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CozeModelInstance coze model表结构体
type CozeModelInstance struct {
	ID          uint              `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt   int64             `gorm:"column:created_at;not null;autoCreateTime:milli;comment:Create Time in Milliseconds" json:"created_at"` // Create Time in Milliseconds
	UpdatedAt   int64             `gorm:"column:updated_at;not null;autoUpdateTime:milli;comment:Update Time in Milliseconds" json:"updated_at"` // Update Time in Milliseconds
	DeletedAt   gorm.DeletedAt    `gorm:"column:deleted_at;comment:Delete Time" json:"deleted_at"`
	Type        int8              `gorm:"column:type;type:smallint;not null;comment:Model Type 0-LLM 1-TextEmbedding 2-Rerank" json:"type"`
	Provider    *ModelProvider    `gorm:"column:provider;not null;serializer:json;comment:Provider Information" json:"provider"`
	DisplayInfo *DisplayInfo      `gorm:"column:display_info;not null;serializer:json;comment:Display Information" json:"display_info"`
	Connection  *Connection       `gorm:"column:connection;not null;serializer:json;comment:Connection Information" json:"connection"`
	Capability  *ModelAbility     `gorm:"column:capability;not null;serializer:json;comment:Model Capability" json:"capability"`
	Parameters  []*ModelParameter `gorm:"column:parameters;not null;serializer:json;comment:Model Parameters" json:"parameters"`
	Extra       datatypes.JSON    `gorm:"column:extra;type:json;comment:Extra Information" json:"extra"`
}

type ModelInstanceList []CozeModelInstance

func (CozeModelInstance) TableName() string {
	return TableNameModelInstance
}

func (l ModelInstanceList) ToMap() map[uint]CozeModelInstance {
	m := make(map[uint]CozeModelInstance)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}

type I18nText struct {
	ZhCn string `thrift:"zh_cn,1" form:"zh_cn" json:"zh_cn" query:"zh_cn"`
	EnUs string `thrift:"en_us,2" form:"en_us" json:"en_us" query:"en_us"`
}

type ModelClass int64

const (
	ModelClass_GPT    ModelClass = 1
	ModelClass_SEED   ModelClass = 2
	ModelClass_Claude ModelClass = 3
	// name: MiniMax
	ModelClass_MiniMax         ModelClass = 4
	ModelClass_Plugin          ModelClass = 5
	ModelClass_StableDiffusion ModelClass = 6
	ModelClass_ByteArtist      ModelClass = 7
	ModelClass_Maas            ModelClass = 9
	// Abandoned: Qianfan (Baidu Cloud)
	ModelClass_QianFan ModelClass = 10
	// name：Google Gemini
	ModelClass_Gemini ModelClass = 11
	// name: Moonshot
	ModelClass_Moonshot ModelClass = 12
	// Name: Zhipu
	ModelClass_GLM ModelClass = 13
	// Name: Volcano Ark
	ModelClass_MaaSAutoSync ModelClass = 14
	// Name: Tongyi Qianwen
	ModelClass_QWen ModelClass = 15
	// name: Cohere
	ModelClass_Cohere ModelClass = 16
	// Name: Baichuan Intelligent
	ModelClass_Baichuan ModelClass = 17
	// Name: ERNIE Bot
	ModelClass_Ernie ModelClass = 18
	// Name: Magic Square
	ModelClass_DeekSeek ModelClass = 19
	// name: Llama
	ModelClass_Llama   ModelClass = 20
	ModelClass_StepFun ModelClass = 23
	ModelClass_Other   ModelClass = 999
)

type ModelProvider struct {
	Name        *I18nText  `thrift:"name,1" form:"name" json:"name" query:"name"`
	IconURI     string     `thrift:"icon_uri,2" form:"icon_uri" json:"icon_uri" query:"icon_uri"`
	IconURL     string     `thrift:"icon_url,3" form:"icon_url" json:"icon_url" query:"icon_url"`
	Description *I18nText  `thrift:"description,4" form:"description" json:"description" query:"description"`
	ModelClass  ModelClass `thrift:"model_class,5" form:"model_class" json:"model_class" query:"model_class"`
}

type DisplayInfo struct {
	Name         string    `thrift:"name,1" form:"name" json:"name" query:"name"`
	Description  *I18nText `thrift:"description,3" form:"description" json:"description" query:"description"`
	OutputTokens int64     `thrift:"output_tokens,4" form:"output_tokens" json:"output_tokens" query:"output_tokens"`
	MaxTokens    int64     `thrift:"max_tokens,5" form:"max_tokens" json:"max_tokens" query:"max_tokens"`
}

type Connection struct {
	BaseConnInfo *BaseConnectionInfo `thrift:"base_conn_info,1" form:"base_conn_info" json:"base_conn_info" query:"base_conn_info"`
}

type ThinkingType int64

const (
	ThinkingType_Default ThinkingType = 0
	ThinkingType_Enable  ThinkingType = 1
	ThinkingType_Disable ThinkingType = 2
	ThinkingType_Auto    ThinkingType = 3
)

type BaseConnectionInfo struct {
	BaseURL      string       `thrift:"base_url,1" form:"base_url" json:"base_url" query:"base_url"`
	APIKey       string       `thrift:"api_key,2" form:"api_key" json:"api_key" query:"api_key"`
	Model        string       `thrift:"model,3" form:"model" json:"model" query:"model"`
	ThinkingType ThinkingType `thrift:"thinking_type,4" form:"thinking_type" json:"thinking_type" query:"thinking_type"`
}

type ModelAbility struct {
	// Do you want to show cot?
	CotDisplay *bool `thrift:"cot_display,1,optional" form:"cot_display" json:"cot_display,omitempty" query:"cot_display"`
	// Supports function calls
	FunctionCall *bool `thrift:"function_call,2,optional" form:"function_call" json:"function_call,omitempty" query:"function_call"`
	// Does it support picture understanding?
	ImageUnderstanding *bool `thrift:"image_understanding,3,optional" form:"image_understanding" json:"image_understanding,omitempty" query:"image_understanding"`
	// Does it support video understanding?
	VideoUnderstanding *bool `thrift:"video_understanding,4,optional" form:"video_understanding" json:"video_understanding,omitempty" query:"video_understanding"`
	// Does it support audio understanding?
	AudioUnderstanding *bool `thrift:"audio_understanding,5,optional" form:"audio_understanding" json:"audio_understanding,omitempty" query:"audio_understanding"`
	// Does it support multimodality?
	SupportMultiModal *bool `thrift:"support_multi_modal,6,optional" form:"support_multi_modal" json:"support_multi_modal,omitempty" query:"support_multi_modal"`
	// Whether to support continuation
	PrefillResp *bool `thrift:"prefill_resp,7,optional" form:"prefill_resp" json:"prefill_resp,omitempty" query:"prefill_resp"`
}

type ModelParamType int64

const (
	ModelParamType_Float   ModelParamType = 1
	ModelParamType_Int     ModelParamType = 2
	ModelParamType_Boolean ModelParamType = 3
	ModelParamType_String  ModelParamType = 4
)

type ModelParamDefaultValue struct {
	DefaultVal string  `thrift:"default_val,1,required" form:"default_val,required" json:"default_val,required" query:"default_val,required"`
	Creative   *string `thrift:"creative,2,optional" form:"creative" json:"creative,omitempty" query:"creative"`
	Balance    *string `thrift:"balance,3,optional" form:"balance" json:"balance,omitempty" query:"balance"`
	Precise    *string `thrift:"precise,4,optional" form:"precise" json:"precise,omitempty" query:"precise"`
}

type Option struct {
	// The value displayed by the option
	Label string `thrift:"label,1" form:"label" json:"label" query:"label"`
	// Filled in value
	Value string `thrift:"value,2" form:"value" json:"value" query:"value"`
}

type ModelParamClass struct {
	// 1="Generation diversity", 2="Input and output length", 3="Output format"
	ClassID int32  `thrift:"class_id,1" form:"class_id" json:"class_id" query:"class_id"`
	Label   string `thrift:"label,2" form:"label" json:"label" query:"label"`
}

type ModelParameter struct {
	// Configuration fields, such as max_tokens
	Name string `thrift:"name,1,required" form:"name,required" json:"name,required" query:"name,required"`
	// Configure field display name
	Label string `thrift:"label,2" form:"label" json:"label" query:"label"`
	// Configuration field detail description
	Desc string `thrift:"desc,3" form:"desc" json:"desc" query:"desc"`
	// type
	Type ModelParamType `thrift:"type,4,required" form:"type,required" json:"type,required" query:"type,required"`
	// Numerical type parameters, the minimum value allowed to be set
	Min string `thrift:"min,5" form:"min" json:"min" query:"min"`
	// Numerical type parameter, the maximum value allowed to be set
	Max string `thrift:"max,6" form:"max" json:"max" query:"max"`
	// Precision of float type parameters
	Precision int32 `thrift:"precision,7" form:"precision" json:"precision" query:"precision"`
	// Parameter default {"default": xx, "creative": xx}
	DefaultVal *ModelParamDefaultValue `thrift:"default_val,8,required" form:"default_val,required" json:"default_val,required" query:"default_val,required"`
	// Enumeration values such as response_format support text, markdown, json
	Options []*Option `thrift:"options,9" form:"options" json:"options" query:"options"`
	// Parameter classification, "Generation diversity", "Input and output length", "Output format"
	ParamClass *ModelParamClass `thrift:"param_class,10" form:"param_class" json:"param_class" query:"param_class"`
}
