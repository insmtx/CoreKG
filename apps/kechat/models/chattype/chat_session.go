package chattype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/insmtx/corekg/pkgs/types"
	"gorm.io/gorm"
)

// 定义资源类型
type ResourceType string

const (
	ResourceTypeModel          ResourceType = "model" // 模型类型
	ResourceTypeAgent          ResourceType = "agent" // Agent 类型
	ResourceTypeForest         ResourceType = "forest"
	ResourceTypeFile           ResourceType = "file"
	ResourceTypeFileList       ResourceType = "file_list"
	ResourceTypeDirList        ResourceType = "dir_list"
	ResourceTypeExcelList      ResourceType = "excel_list"
	ResourceTypeExcelSheetList ResourceType = "excel_sheet_list"
	ResourceTypeDBList         ResourceType = "db_list"
	ResourceTypeDBTableList    ResourceType = "db_table_list"
	ResourceTypeExternalData   ResourceType = "external_data"

	ResourceTypeReactExcelList ResourceType = "react_excel_list"

	ResourceTypeGraphSearch ResourceType = "graph_search"
)
const DefaultSessionName = "New Chat"

type ResourceQASessionBaseType string

const (
	ResourceQASessionBaseTypeStandard ResourceQASessionBaseType = "standard"
	ResourceQASessionBaseTypeExcel    ResourceQASessionBaseType = "excel"
	ResourceQASessionBaseTypeDbMYSQL  ResourceQASessionBaseType = "mysql"
	ResourceQASessionBaseModel        ResourceQASessionBaseType = "model"         // 模型类型
	ResourceQASessionBaseAgent        ResourceQASessionBaseType = "agent"         // Agent 类型
	ResourceQASessionBaseExternalData ResourceQASessionBaseType = "external_data" // 外部数据源类型

	ResourceQASessionBaseGraphQA ResourceQASessionBaseType = "graph_qa" // 图谱洞察模式

	ResourceQASessionBaseTypeReactExcel  ResourceQASessionBaseType = "react_excel"
	ResourceQASessionBaseTypeForestAgent ResourceQASessionBaseType = "forest_agent"

	ResourceQASessionBaseTypeGraphSearch ResourceQASessionBaseType = "graph_search"
)

type SessionSubjectType string

const (
	SessionSubjectTypeProject SessionSubjectType = "project"
)

// ChatSession 聊天会话
type ChatSession struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	// Name 会话名称
	Name string `gorm:"column:name;type:varchar(255);not null;index:name" json:"name"`
	// ModelName 模型名称
	ModelName string `gorm:"column:model_name;type:varchar(255);not null;index:model_name" json:"model_name"`
	// ModelID 模型ID
	ModelID uint `gorm:"column:model_id;type:int;not null;default:0;index:model_id;comment:模型ID" json:"model_id"`
	//BaseAgentID 机器人ID
	BaseAgentID uint `gorm:"column:base_agent_id;type:int;not null;default:0;index:base_agent_id;comment:机器人ID" json:"base_agent_id"`
	// Type 资源类型，用于区分是 Model 还是 Agent
	ResourceType ResourceType              `gorm:"column:resource_type;type:varchar(255);not null;index:resource_type;comment:'资源类型，用于区分是 Model 还是 Agent'" json:"resource_type"`
	BaseType     ResourceQASessionBaseType `gorm:"column:base_type;type:varchar(32);default:'standard';comment:'基础类型，standard：标准, data_excel：Excel, data_mysql：MySQL'" json:"mode"`
	// AgentVersion 机器人版本
	AgentVersion uint `gorm:"column:agent_version;type:int;default:0;index:agent_version;comment:机器人版本" json:"agent_version"`
	// Input 输入参数
	Input InputList `gorm:"column:input;type:json;comment:'输入参数'" json:"input"`
	// IsTop 是否置顶
	IsTop types.Bool `gorm:"column:is_top;type:smallint;not null;default:-1;comment:'是否置顶'" json:"is_top"`
	// 关联文件id
	FileID uint `gorm:"type:int;column:file_id;not null;default:0;comment:file_id" json:"file_id"`
	// 关联多文件id
	FileIDList types.UintArray `gorm:"column:file_id_list;type:text;comment:'文件ID列表'" json:"file_id_list"`
	// 关联多库id
	ForestIDList     types.UintArray   `gorm:"column:forest_id_list;type:text;comment:'森林ID列表'" json:"forest_id_list"`
	ExcelIDList      types.UintArray   `gorm:"column:excel_id_list;type:text;comment:'excelIDList'" json:"excel_id_list"`
	ExcelSheetIDList types.UintArray   `gorm:"column:excel_sheet_id_list;type:text;comment:'excelSheetIDList'" json:"excel_sheet_id_list"`
	DBList           types.StringArray `gorm:"column:db_list;type:text;comment:'数据库名称列表'" json:"db_list"`
	DBTableList      types.StringArray `gorm:"column:db_table_list;type:text;comment:'数据表名称列表'" json:"db_table_list"`
	// es索引
	EsIndex string `gorm:"column:es_index;type:varchar(255);default:'ke_0';comment:'es索引'" json:"es_index"`
	// external id
	ExternalID string `gorm:"column:external_id;type:varchar(127);comment:'外部调用标识'" json:"external_id"`

	SubjectID uint `gorm:"column:subject_id;type:bigint;not null;default:0;comment:'主体id';index" json:"subject_id"`

	ExternalTokenIDList types.UintArray `gorm:"column:external_token_id_list;type:text;comment:'外部数据源id'" json:"external_token_id_list"`

	PromptMode string `gorm:"column:prompt_mode;type:varchar(32);default:'';comment:'prompt模式'" json:"prompt_mode"`
}

type ChatSessionList []ChatSession

func (ChatSession) TableName() string {
	return TableNameChatSessions
}

type Input struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type InputList []Input

func (ep InputList) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *InputList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for ExamPosition")
	}
	return json.Unmarshal(bytes, ep)
}
