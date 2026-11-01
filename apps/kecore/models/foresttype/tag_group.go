package foresttype

import (
	"gorm.io/gorm"
)

type TagGroupStatus string

const (
	TagGroupStatusEnable  TagGroupStatus = "enable"
	TagGroupStatusDisable TagGroupStatus = "disable"
)

// TagGroup 标签分组表结构体
type TagGroup struct {
	gorm.Model
	CompanyID  uint           `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司 id"`
	CreatedUin uint           `gorm:"column:created_uin;type:bigint;not null;;comment:创建人uin"`
	Name       string         `gorm:"column:name;type:varchar(64);not null;;comment:分组名称"`
	Status     TagGroupStatus `gorm:"column:status;type:varchar(16);not null;default enable;comment:状态：enable-启用，disable-禁用"`
	UpdatedUin uint           `gorm:"column:updated_uin;type:bigint;not null;;comment:更新人uin"`
}

type TagGroupList []TagGroup

func (TagGroup) TableName() string {
	return TableNameTagGroup
}

func (l TagGroupList) ToMap() map[uint]TagGroup {
	m := make(map[uint]TagGroup)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
