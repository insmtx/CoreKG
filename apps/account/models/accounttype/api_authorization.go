package accounttype

import (
	"time"

	"gorm.io/gorm"
)

// APIAuthorization API授权
type APIAuthorization struct {
	gorm.Model

	// 公司ID
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0" json:"company_id"`
	ApiID     uint `gorm:"column:api_id;type:bigint;not null;default:0" json:"api_id"`

	// Status 状态
	Status APIStatus `gorm:"column:status;type:varchar(20);not null;default:'normal'" json:"status"`
	// ExpiredAt 过期时间
	ExpiredAt *time.Time `gorm:"column:expired_at;type:timestamptz;comment:过期时间" json:"expired_at"`
}

// TableName 表名
func (APIAuthorization) TableName() string {
	return TableNameAPIAuthorization
}

// IsValid 是否有效
func (a APIAuthorization) IsValid() bool {
	return a.Status == APIStatusNormal && !a.IsExpired()
}

// IsExpired 是否过期
func (a APIAuthorization) IsExpired() bool {
	if a.ExpiredAt != nil && a.ExpiredAt.Before(time.Now()) {
		return true
	}
	return false
}
