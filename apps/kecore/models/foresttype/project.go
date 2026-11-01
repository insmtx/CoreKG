package foresttype

import (
	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

type ProjectType string

const (
	ProjectTypeCustom   ProjectType = "custom"
	ProjectTypeForestQA ProjectType = "forest_qa"
	ProjectTypeAgentQA  ProjectType = "agent_qa"
)

const (
	ProjectSortForestQA = 2
	ProjectSortAgentQA  = 1
	ProjectSortCustom   = 0
)

type KnownowProject struct {
	gorm.Model
	Uin          uint            `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID    uint            `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	Name         string          `gorm:"type:varchar(255);column:name;not null;default:'';comment:'项目名'" json:"name"`
	ForestIDList types.UintArray `gorm:"column:forest_id_list;type:text;comment:'森林ID列表'" json:"forest_id_list"`
	ProjectType  ProjectType     `gorm:"column:project_type;type:varchar(32);not null;default:'custom';comment:'项目类型：custom=自定义, forest_qa=知识库问答, agent_qa=智能体问答'" json:"project_type"`
	Sort         uint            `gorm:"column:sort;type:smallint;not null;default:0;comment:'排序权重：forest_qa=2, agent_qa=1, custom=0'" json:"sort"`
}

type KnownowProjectList []KnownowProject

func (KnownowProject) TableName() string {
	return TableNameKeProject
}
