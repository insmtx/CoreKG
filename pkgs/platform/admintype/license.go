package admintype

import (
	"time"

	"github.com/insmtx/corekg/apps/corekg/models/license"
	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

const (
	// TableNameLicense 运营端许可证表
	TableNameLicense = "admin_license"

	// TableNameDailyLog 校验日志表
	TableNameDailyLog = "core_daily_log"
)

// DailyLog defines a single daily log entry
type DailyLog struct {
	gorm.Model
	//log date
	Date string `gorm:"column:date;primaryKey" json:"date"`
	//pre-date hash
	PreviousHash string `gorm:"column:previous_hash;type:varchar(511)" json:"previous_hash"`
	//current-date hash
	CurrentHash string `gorm:"column:current_hash;type:varchar(511)" json:"current_hash"`
	//verify result
	Valid types.Bool `gorm:"column:valid" json:"valid"`
	//some note message for this log entry
	Message string `gorm:"column:message;type:text" json:"message"`
}

func (DailyLog) TableName() string {
	return TableNameDailyLog
}

// License 运营端许可证
type License struct {
	gorm.Model
	Subject    string          `gorm:"column:subject;type:varchar(255);comment:签发主体" json:"subject"`
	Env        license.EnvType `gorm:"column:env;type:varchar(127);comment:环境类型" json:"env"`
	UID        string          `gorm:"column:uid;type:varchar(255);comment:主体环境UID" json:"uid"`
	ExpiredAt  *time.Time      `gorm:"column:expired_at;type:timestamptz;comment:license到期时间" json:"expired_at"`
	Issuer     string          `gorm:"column:issuer;type:varchar(255);comment:签发人" json:"issuer"`
	Serial     string          `gorm:"column:serial;type:varchar(255);comment:License序列号" json:"serial"`
	Meta       Meta            `gorm:"column:meta;type:varchar(1023);comment:license元信息;serializer:json" json:"meta"`
	Raw        string          `gorm:"column:raw;type:text;comment:license信息" json:"raw"`
	PrivateKey string          `gorm:"column:private_key;type:text;comment:license私钥" json:"private_key"`
	PublicKey  string          `gorm:"column:public_key;type:text;comment:license公钥" json:"public_key"`
	Note       string          `gorm:"column:note;type:text;comment:license备注" json:"note"`
	VersionKey string          `gorm:"column:version_key;type:varchar(127);comment:版本类型" json:"version_key"`
}

func (License) TableName() string {
	return TableNameLicense
}

// Meta license元信息
type Meta struct {
	//license id
	ID uint `json:"id"`
	//序列号
	Serial string `json:"serial"`
	//环境类型
	Env license.EnvType `json:"env"`
	//环境唯一标识
	UID string `json:"uid"`
	//主体
	Subject string `json:"subject"`
	//签发人
	Issuer string `json:"issuer"`
	//创建时间
	CreatedAt time.Time `json:"created_at"`
	//到期时间
	ExpiredAt time.Time `json:"expired_at"`
	//随机种子 Used to generate the HMAC key
	Seed string `json:"seed"`
	//版本类型
	VersionKey string `json:"version_key"`
}
