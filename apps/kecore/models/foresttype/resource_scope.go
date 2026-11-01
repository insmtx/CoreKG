package foresttype

import "gorm.io/gorm"

type (
	ActionType   string
	ResourceType string
)

var (
	//操作类型

	// 管理
	ActionManage ActionType = "manage"
	// 查看
	ActionView ActionType = "view"
	// 禁止
	ActionBan ActionType = "ban"

	//资源类型

	ResourceTypeForest     ResourceType = "forest"
	ResourceTypeAgent      ResourceType = "agent"
	ResourceTypeGraph      ResourceType = "graph"
	ResourceTypeArticle    ResourceType = "article"
	ResourceTypeDB         ResourceType = "db"
	ResourceTypeExcel      ResourceType = "excel"
	ResourceTypeForestFile ResourceType = "forest_file"
	ResourceTypeQAPair     ResourceType = "qa_pair"
	ResourceTypeWorkflow   ResourceType = "workflow"
	ResourceTypeApp        ResourceType = "app"
	ResourceTypePlugin     ResourceType = "plugin"
	ResourceTypePrompt     ResourceType = "prompt"
)

type KeResourceScope struct {
	gorm.Model
	ResourceType ResourceType `gorm:"column:resource_type;type:varchar(24);not null;default:'';comment:资源类型" json:"resource_type"`
	ResourceID   uint         `gorm:"column:resource_id;type:bigint;not null;default:0;comment:资源ID" json:"resource_id"`
	ScopeType    ScopeType    `gorm:"column:scope_type;type:varchar(24);not null;default:'';comment:作用域类型" json:"scope_type"`

	ScopeID uint       `gorm:"column:scope_id;type:bigint;not null;default:0;comment:作用域对象ID" json:"scope_id"`
	Action  ActionType `gorm:"column:action;type:varchar(24);not null;default:'';comment:操作类型" json:"action"`
}

type KeResourceScopeList []KeResourceScope

func (KeResourceScope) TableName() string {
	return TableNameKeResourceScope
}

const (
	CozeResourceTypeAccount             ResourceType = "1"
	CozeResourceTypeWorkspace           ResourceType = "2"
	CozeResourceTypeApp                 ResourceType = "3"
	CozeResourceTypeAgent               ResourceType = "4"
	CozeResourceTypePlugin              ResourceType = "5"
	CozeResourceTypeWorkflow            ResourceType = "6"
	CozeResourceTypeKnowledge           ResourceType = "7"
	CozeResourceTypePersonalAccessToken ResourceType = "8"
	CozeResourceTypeConnector           ResourceType = "9"
	CozeResourceTypeCard                ResourceType = "10"
	CozeResourceTypeCardTemplate        ResourceType = "11"
	CozeResourceTypeConversation        ResourceType = "12"
	CozeResourceTypeFile                ResourceType = "13"
	CozeResourceTypeServicePrincipal    ResourceType = "14"
	CozeResourceTypeEnterprise          ResourceType = "15"
	CozeResourceTypeMigrateTask         ResourceType = "16"
	CozeResourceTypePrompt              ResourceType = "17"
	CozeResourceTypeUI                  ResourceType = "18"
	CozeResourceTypeProject             ResourceType = "19"
	CozeResourceTypeEvaluationDataset   ResourceType = "20"
	CozeResourceTypeEvaluationTask      ResourceType = "21"
	CozeResourceTypeEvaluator           ResourceType = "22"
	CozeResourceTypeDatabase            ResourceType = "23"
	CozeResourceTypeOceanProject        ResourceType = "24"
	CozeResourceTypeFinetuneTask        ResourceType = "25"
	CozeResourceTypeKnowledgeDocument   ResourceType = "26"
	CozeResourceTypeKnowledgeSlice      ResourceType = "27"
)

var ResourceTypeMap = map[ResourceType]ResourceType{
	ResourceTypeForest:       ResourceTypeForest,
	ResourceTypeAgent:        ResourceTypeAgent,
	ResourceTypeGraph:        ResourceTypeGraph,
	ResourceTypeArticle:      ResourceTypeArticle,
	ResourceTypeDB:           ResourceTypeDB,
	ResourceTypeExcel:        ResourceTypeExcel,
	ResourceTypeQAPair:       ResourceTypeQAPair,
	CozeResourceTypeApp:      ResourceTypeApp,
	CozeResourceTypeAgent:    ResourceTypeAgent,
	CozeResourceTypeWorkflow: ResourceTypeWorkflow,
	CozeResourceTypePrompt:   ResourceTypePrompt,
	CozeResourceTypePlugin:   ResourceTypePlugin,
}
