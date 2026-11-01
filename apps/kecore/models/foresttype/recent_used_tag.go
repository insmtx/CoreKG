package foresttype

import (
	"time"

	"gorm.io/gorm"
)

// RecentUsedTag 最近使用标签表表结构体
type RecentUsedTag struct {
	gorm.Model
	CompanyID  uint      `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司 id"`
	GroupID    uint      `gorm:"column:group_id;type:bigint;not null;;comment:标签分组ID"`
	LastUsedAt time.Time `gorm:"column:last_used_at;type:timestamptz;not null;default CURRENT_TIMESTAMP(3);comment:最后使用时间"`
	TagID      uint      `gorm:"column:tag_id;type:bigint;not null;;comment:标签ID"`
	Uin        uint      `gorm:"column:uin;type:bigint;not null;;comment:用户uin"`
	UsageCount int32     `gorm:"column:usage_count;type:bigint;not null;default 1;comment:使用次数"`
}

type RecentUsedTagList []RecentUsedTag

func (RecentUsedTag) TableName() string {
	return TableNameRecentUsedTag
}

func (l RecentUsedTagList) ToMap() map[uint]RecentUsedTag {
	m := make(map[uint]RecentUsedTag)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
