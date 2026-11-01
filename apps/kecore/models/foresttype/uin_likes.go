package foresttype

import (
	"gorm.io/gorm"
)

// UinLikes 点赞表结构体
type UinLikes struct {
	gorm.Model
	CompanyID    uint         `gorm:"column:company_id;type:bigint;not null;;comment:公司id"`
	ResourceID   uint         `gorm:"column:resource_id;type:bigint;not null;;comment:资源id"`
	ResourceType ResourceType `gorm:"column:resource_type;type:varchar(255);not null;;comment:资源类型"`
	Uin          uint         `gorm:"column:uin;type:bigint;not null;;comment:用户uin"`
}

type UinLikesList []UinLikes

func (UinLikes) TableName() string {
	return TableNameUinLikes
}

func (l UinLikesList) ToMap() map[uint]UinLikes {
	m := make(map[uint]UinLikes)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
