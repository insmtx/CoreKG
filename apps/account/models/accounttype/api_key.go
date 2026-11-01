package accounttype

import (
	"time"

	"gorm.io/gorm"
)

// AccessKeyStatus 访问凭证状态
type AccessKeyStatus string

const (
	// AccessKeyStatusNormal 正常
	AccessKeyStatusNormal AccessKeyStatus = "normal"
	// AccessKeyStatusDisabled 禁用
	AccessKeyStatusDisabled AccessKeyStatus = "disabled"
)

type ResourceType string

var (
	ResourceTypeAgent ResourceType = "agent"
)

const (
	APIKeyNameSystem    = "prod"
	APIKeyPurposeSystem = "prod"
)

// APIKey API key
type APIKey struct {
	gorm.Model

	// Uin
	Uin uint `gorm:"column:uin;type:bigint;not null;comment:uin"`
	// 公司ID
	CompanyID uint `gorm:"column:company_id;type:bigint;index:company_id" json:"company_id"`
	// Name Key名称
	Name string `gorm:"column:name;type:varchar(255);not null;index:name" json:"name"`
	// Key 密钥
	APIKey string `gorm:"column:api_key;type:varchar(255);not null;uniqueIndex:idx_api_key" json:"api_key"`
	// purpose 用途
	Purpose string `gorm:"column:purpose;type:varchar(255)" json:"purpose"`
	//ResourceType
	ResourceType ResourceType `gorm:"column:resource_type;type:varchar(255)" json:"resource_type"`
	// resource id
	ResourceID uint `gorm:"column:resource_id;type:varchar(255)" json:"resource_id"`

	// Status 状态
	Status AccessKeyStatus `gorm:"column:status;type:varchar(11);not null;default:'normal'" json:"status"`
	// ExpiredAt 过期时间
	ExpiredAt *time.Time `gorm:"column:expired_at;type:timestamptz;comment:过期时间" json:"expired_at"`
}

type APIKeyList []APIKey

// TableName 表名
func (APIKey) TableName() string {
	return TableNameAPIKey
}
func (k *APIKey) IsExpired() bool {
	if k.ExpiredAt == nil {
		// nil 表示永不过期
		return false
	}
	// 比较当前时间
	return time.Now().After(*k.ExpiredAt)
}

type APIKeyPrivilege struct {
	gorm.Model
	ApiKeyID uint `gorm:"column:api_key_id;type:bigint;not null;index:api_key_id" json:"api_key_id"`
	ApiID    uint `gorm:"column:api_id;type:bigint;not null;index:api_id" json:"api_id"`
}

// TableName 表名
func (APIKeyPrivilege) TableName() string {
	return TableNameAPIKeyPrivilege
}
