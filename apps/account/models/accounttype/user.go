package accounttype

import (
	"github.com/insmtx/corekg/pkgs/types"
	"gorm.io/gorm"
)

// User 用户登录信息表
type User struct {
	gorm.Model

	// Identify 用户标识, 注册后不可修改 TODO release-1.7废弃
	Identify string `gorm:"column:identify;type:varchar(256);uniqueIndex;not null;comment:客户标识"`

	Name      string `gorm:"column:name;type:varchar(64);not null;comment:客户名称"`
	Bio       string `gorm:"column:bio;type:varchar(256);comment:客户简介"`
	AvatarURL string `gorm:"column:avatar_url;type:varchar(256);comment:客户头像"`

	Email    *string `gorm:"column:email;type:varchar(64);comment:客户邮箱"`
	Phone    *string `gorm:"column:phone;type:varchar(16);comment:客户手机"`
	Password *string `gorm:"column:password;type:varchar(128)" json:"password"`
	//是否修改过密码(1:是,-1:否)
	PasswordChanged types.Bool `gorm:"column:password_changed;type:smallint;comment:是否修改过密码(1:是,-1:否)" json:"password_changed"`

	// ThirdParty
	GithubID         *uint   `gorm:"column:github_id;type:bigint;uniqueIndex;comment:Github ID"`
	WorkWechatUserID *string `gorm:"column:work_wechat_user_id;type:varchar(64);uniqueIndex;comment:企业微信用户ID"`
	// WechatUnionID 微信开放平台的UnionID
	WechatUnionID *string `gorm:"column:wechat_union_id;type:varchar(64);uniqueIndex;comment:微信用户UnionID"`
	// WechatWebOpenID 微信开放平台网页APP的OpenID
	WechatWebOpenID *string `gorm:"column:wechat_web_open_id;type:varchar(64);uniqueIndex;comment:微信用户WebOpenID"`
	// CompanyQuota 公司配额
	CompanyQuota uint `gorm:"column:company_quota;type:bigint;default:2;comment:公司配额" json:"company_quota"`
}

type UserList []User

// TableName 表名
func (User) TableName() string {
	return TableNameUser
}
