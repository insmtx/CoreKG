package tokenmgr

import (
	"time"

	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	dbtools "github.com/ygpkg/yg-go/dbtools/v2"
	_ "github.com/ygpkg/yg-go/dbtools/v2/mysqldrv"
	"gorm.io/gorm"
)

const TableNameUserExternalBinding = "account_external_binding"

type Platform string

const (
	PlatformGithub      Platform = "github"
	PlatformGoogle      Platform = "google"
	PlatformSlack       Platform = "slack"
	PlatformGmail       Platform = "gmail"
	PlatformGoogleDrive Platform = "google-drive"
	PlatformConfluence  Platform = "confluence"
)

// TableName return table name
type ExternalToken struct {
	gorm.Model

	Uin          uint       `gorm:"not null;index:idx_uin_provider;comment:系统用户唯一标识" json:"uin"`
	CompanyID    uint       `gorm:"not null;comment:公司ID/组织ID" json:"company_id"`
	Platform     Platform   `gorm:"type:varchar(50);not null;comment:第三方平台标识，如 github/google/slack" json:"platform"`
	Provider     string     `gorm:"type:varchar(50);index:idx_uin_provider;comment:平台下的服务/提供者，如 gmail/drive" json:"provider"`
	ExternalID   string     `gorm:"type:varchar(100);not null;comment:第三方平台用户ID" json:"external_id"`
	AccessToken  string     `gorm:"type:text;not null;comment:加密存储的 access_token" json:"access_token"`
	RefreshToken string     `gorm:"type:text;comment:加密存储的 refresh_token" json:"refresh_token"`
	ExpiresAt    *time.Time `gorm:"type:timestamptz;comment:access_token 过期时间" json:"expires_at"`
	Status       int8       `gorm:"not null;default:1;comment:绑定状态: 1=绑定 0=解绑/失效" json:"status"`
	Email        string     `gorm:"type:varchar(255);comment:第三方平台邮箱" json:"email"`
	Avatar       string     `gorm:"type:varchar(500);comment:第三方平台头像 URL" json:"avatar"`
}

type TokenInfo struct {
	ExternalID   string    `json:"externalID"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	TokenType    string    `json:"tokenType,omitempty"`
	Provider     string    `json:"provider,omitempty"`
}

// TableName 表名
func (ExternalToken) TableName() string {
	return TableNameUserExternalBinding
}

func InitDB() error {
	return dbtools.InitModel(dbutil.Account(),
		&ExternalToken{},
	)
}

// encryptToken 加密token
func encryptToken(token string) (string, error) {
	// TODO
	return token, nil
}

// decryptToken 解密token
func decryptToken(encryptedToken string) (string, error) {
	// TODO
	return encryptedToken, nil
}
