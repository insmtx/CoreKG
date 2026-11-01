package login_setting

import "gorm.io/gorm"

const (
	tableNameProfix       = "admin_"
	TableNameLoginSetting = tableNameProfix + "login_setting"
)

// EnvType  环境
type EnvType string

const (
	// EnvTest 测试
	EnvTest EnvType = "test"
	// EnvProd 生产
	EnvProd EnvType = "prod"
)

// type Method string

// const (
// 	MethodWX    Method = "wx"
// 	MethodWXCom Method = "wxcom"
// 	MethodPhone Method = "phone"
// )

// LoginSetting login接口配置
type LoginSetting struct {
	gorm.Model
	// DomainName 域名
	DomainName string `gorm:"type:varchar(255);not null" json:"domain_name"`
	// Path 路径
	Path string `gorm:"type:varchar(128);unique_index" json:"path"`
	// Env 环境
	Env EnvType `gorm:"type:varchar(128);" json:"env"`
	// // Methods 登录方式
	// Methods []Method `gorm:"type:text;not null;serializer:json" json:"methods"`
	IsEnableWeChat    bool `gorm:"type:smallint;not null" json:"is_enable_wechat"`
	IsEnableWeChatCom bool `gorm:"type:smallint;not null" json:"is_enable_wechatcom"`
	IsEnablePhone     bool `gorm:"type:smallint;not null" json:"is_enable_phone"`
	IsEnableEmail     bool `gorm:"type:smallint;not null" json:"is_enable_email"`
	IsEnablePassword  bool `gorm:"type:smallint;not null;default:0" json:"is_enable_password"`
	// Title 标题
	Title string `gorm:"type:varchar(255);not null" json:"title"`
	// ImageUrl 图片链接
	ImageUrl string `gorm:"type:varchar(255);not null" json:"image_url"`

	// 微信登录appid
	AppID string `gorm:"type:varchar(255);not null" json:"appid"`
	// 企微登录appid
	AppIDCom string `gorm:"type:varchar(255);not null" json:"appid_com"`
	// 企微agentid，自建项目使用
	AgentID string `gorm:"type:varchar(255);not null" json:"agentid"`
	// 运行注册
	AllowRegister bool `gorm:"type:smallint;not null" json:"allow_register"`

	// 签发人信息
	Issuer string `gorm:"type:varchar(128);not null" json:"issuer"`
	// key
	AuthKey string `gorm:"type:varchar(128);not null" json:"auth_key"`
	// loginUrl
	LoginURL string `gorm:"type:varchar(32);not null" json:"login_url"`
}

// TableName 表名
func (LoginSetting) TableName() string {
	return TableNameLoginSetting
}
