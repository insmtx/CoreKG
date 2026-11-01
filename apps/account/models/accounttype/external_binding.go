package accounttype

import (
	"time"

	"gorm.io/gorm"
)

// AccountExternalBinding 用户第三方平台授权绑定记录表
type ExternalBinding struct {
	gorm.Model

	Uin          uint       `gorm:"not null;index:idx_user_provider;comment:系统用户ID" json:"uin"`
	CompanyID    uint       `gorm:"not null;comment:公司ID/组织ID" json:"company_id"`
	Platform     string     `gorm:"type:varchar(50);not null;comment:第三方平台标识，如 github/google/slack" json:"platform"`
	Provider     string     `gorm:"type:varchar(50);index:idx_user_provider;comment:平台下的服务/提供者，如 gmail/drive" json:"provider"`
	ExternalID   string     `gorm:"type:varchar(100);not null;comment:第三方平台用户ID" json:"external_id"`
	AccessToken  string     `gorm:"type:text;not null;comment:加密存储的 access_token" json:"access_token"`
	RefreshToken string     `gorm:"type:text;comment:加密存储的 refresh_token" json:"refresh_token"`
	ExpiresAt    *time.Time `gorm:"type:timestamptz;comment:access_token 过期时间" json:"expires_at"`
	Status       int8       `gorm:"not null;default:1;comment:绑定状态: 1=绑定 0=解绑/失效" json:"status"`
	Email        string     `gorm:"type:varchar(255);comment:第三方平台邮箱" json:"email"`
	Avatar       string     `gorm:"type:varchar(500);comment:第三方平台头像 URL" json:"avatar"`
}

// TableName 表名
func (ExternalBinding) TableName() string {
	return TableNameUserExternalBinding
}
