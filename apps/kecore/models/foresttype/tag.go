package foresttype

import (
	"gorm.io/gorm"
)

// Tag 标签表结构体
type Tag struct {
	gorm.Model
	CompanyID  uint           `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司 id"`
	CreatedUin uint           `gorm:"column:created_uin;type:bigint;not null;;comment:创建人uin"`
	GroupID    uint           `gorm:"column:group_id;type:bigint;not null;;comment:标签分组ID"`
	Name       string         `gorm:"column:name;type:varchar(64);not null;;comment:标签名称"`
	Status     TagGroupStatus `gorm:"column:status;type:varchar(16);not null;default enable;comment:状态：enable-启用，disable-禁用"`
	UpdatedUin uint           `gorm:"column:updated_uin;type:bigint;not null;;comment:更新人uin"`
}

type TagList []Tag

func (Tag) TableName() string {
	return TableNameTag
}

func (l TagList) ToMap() map[uint]Tag {
	m := make(map[uint]Tag)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
