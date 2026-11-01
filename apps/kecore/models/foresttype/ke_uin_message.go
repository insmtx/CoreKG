package foresttype

import (
	"time"

	"gorm.io/gorm"
)

type MessageReadStatus string

const (
	MessageReadStatusUnread MessageReadStatus = "unread"
	MessageReadStatusRead   MessageReadStatus = "read"
)

type MessageSourceType string

const (
	MessageSourceTypeCompany      MessageSourceType = "company"
	MessageSourceTypeAnnouncement MessageSourceType = "announcement"
	MessageSourceTypeForestFile   MessageSourceType = "forest_file"
)

// KeUinMessage 用户消息表结构体
type KeUinMessage struct {
	gorm.Model
	CompanyID    uint                `gorm:"column:company_id;type:bigint;not null;;comment:公司ID"`
	UserID       uint                `gorm:"column:user_id;type:bigint;not null;;comment:用户ID"`
	Uin          uint                `gorm:"column:uin;type:bigint;not null;;comment:uin"`
	TemplateID   uint                `gorm:"column:template_id;type:bigint;not null;;comment:模板ID"`
	Title        string              `gorm:"column:title;type:varchar(256);not null;;comment:消息标题"`
	Content      string              `gorm:"column:content;type:varchar(256);not null;;comment:渲染后的消息内容"`
	TemplateType MessageTemplateType `gorm:"column:template_type;type:varchar(32);not null;;comment:模板类型：system-系统消息，announcement-公告消息"`
	SourceType   MessageSourceType   `gorm:"column:source_type;type:varchar(64);;;comment:业务关联类型"`
	SourceID     uint                `gorm:"column:source_id;type:bigint;;;comment:业务关联ID"`
	RoutePath    string              `gorm:"column:route_path;type:varchar(256);;;comment:实际跳转路由路径"`
	ReadStatus   MessageReadStatus   `gorm:"column:read_status;type:varchar(16);not null;default unread;comment:已读状态：unread-未读，read-已读"`
	ReadAt       *time.Time          `gorm:"column:read_at;type:timestamptz;;;comment:阅读时间"`
}

type KeUinMessageList []KeUinMessage

func (KeUinMessage) TableName() string {
	return TableNameKeUinMessage
}

func (l KeUinMessageList) ToMap() map[uint]KeUinMessage {
	m := make(map[uint]KeUinMessage)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
