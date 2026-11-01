package admintype

import "gorm.io/gorm"

const (
	// TableNameAdminAnnouncement 系统公告表
	TableNameAdminAnnouncement = "admin_announcement"
)

// AdminAnnouncement 系统公告表表结构体
type AdminAnnouncement struct {
	gorm.Model
	Uin       uint   `gorm:"column:uin;type:bigint;not null;;comment:创建人uin"`
	CompanyID uint   `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司ID"`
	Creator   string `gorm:"column:creator;type:varchar(511);not null;;comment:创建人昵称"`
	Tag       string `gorm:"column:tag;type:varchar(127);not null;;comment:版本tag"`
	Content   string `gorm:"column:content;type:text;;comment:公告内容"`
}

type AdminAnnouncementList []AdminAnnouncement

func (AdminAnnouncement) TableName() string {
	return TableNameAdminAnnouncement
}

func (l AdminAnnouncementList) ToMap() map[uint]AdminAnnouncement {
	m := make(map[uint]AdminAnnouncement)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
