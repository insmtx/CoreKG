package apptype

import (
	"time"

	"gorm.io/gorm"
)

type KeApplication struct {
	gorm.Model
	Uin            uint            `gorm:"column:uin;type:bigint;not null;default:0;index:idx_app_uin" json:"uin"`
	CompanyID      uint            `gorm:"column:company_id;type:bigint;not null;default:0;index:idx_app_company" json:"company_id"`
	Name           string          `gorm:"column:name;type:varchar(255);not null;default:''" json:"name"`
	Type           AppTemplateType `gorm:"column:type;type:varchar(64);not null;default:''" json:"type"`
	Status         AppStatus       `gorm:"column:status;type:varchar(32);not null;default:'draft'" json:"status"`
	Description    string          `gorm:"column:description;type:varchar(1024);not null;default:''" json:"description"`
	Icon           string          `gorm:"column:icon;type:varchar(512);not null;default:''" json:"icon"`
	Color          string          `gorm:"column:color;type:varchar(16);not null;default:'#0C99FF'" json:"color"`
	KnowledgeCount int             `gorm:"column:knowledge_count;type:int;not null;default:0" json:"knowledge_count"`
	FAQCount       int             `gorm:"column:faq_count;type:int;not null;default:0" json:"faq_count"`
	SyncStatus     SyncStatus      `gorm:"column:sync_status;type:varchar(32);not null;default:'success'" json:"sync_status"`
	LastSyncAt     *time.Time      `gorm:"column:last_sync_at;type:timestamptz" json:"last_sync_at"`
	LastPublishAt  *time.Time      `gorm:"column:last_publish_at;type:timestamptz" json:"last_publish_at"`
	Config         AppConfig       `gorm:"column:config;type:json" json:"config"`
	ForestID       *uint           `gorm:"column:forest_id;type:bigint" json:"forest_id"`
	CrawlConfig    string          `gorm:"column:crawl_config;type:json" json:"crawl_config"`
}

func (KeApplication) TableName() string { return TableNameKeApplication }

type KeApplicationList []*KeApplication
