package foresttype

import (
	"gorm.io/gorm"
)

// UinCollections 收藏表结构体
type UinCollections struct {
	gorm.Model
	CompanyID    uint         `gorm:"column:company_id;type:bigint;not null;;comment:公司id"`
	ResourceID   uint         `gorm:"column:resource_id;type:bigint;not null;;comment:资源id"`
	ResourceType ResourceType `gorm:"column:resource_type;type:varchar(255);not null;;comment:资源类型"`
	Uin          uint         `gorm:"column:uin;type:bigint;not null;;comment:用户uin"`
}

type UinCollectionsList []UinCollections

func (UinCollections) TableName() string {
	return TableNameUinCollections
}

func (l UinCollectionsList) ToMap() map[uint]UinCollections {
	m := make(map[uint]UinCollections)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
