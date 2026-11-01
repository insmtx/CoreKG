package foresttype

import (
	"gorm.io/gorm"
)

type TagResourceType string

const (
	TagResourceTypeFile TagResourceType = "file"
)

// ResourceTag 资源标签关联表表结构体
type ResourceTag struct {
	gorm.Model
	CompanyID    uint            `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司 id"`
	CreatedUin   uint            `gorm:"column:created_uin;type:bigint;not null;;comment:创建人uin"`
	GroupID      uint            `gorm:"column:group_id;type:bigint;not null;;comment:标签分组ID"`
	ResourceID   uint            `gorm:"column:resource_id;type:bigint;not null;;comment:资源 id"`
	ResourceType TagResourceType `gorm:"column:resource_type;type:varchar(127);not null;;comment:资源类型，file：知识库文件"`
	TagID        uint            `gorm:"column:tag_id;type:bigint;not null;;comment:标签ID"`
	UpdatedUin   uint            `gorm:"column:updated_uin;type:bigint;not null;;comment:更新人uin"`
}

type ResourceTagList []ResourceTag

func (ResourceTag) TableName() string {
	return TableNameResourceTag
}

func (l ResourceTagList) ToMap() map[uint]ResourceTag {
	m := make(map[uint]ResourceTag)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
