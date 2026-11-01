package accounttype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/insmtx/corekg/apps/account/models/perm"
	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

type CompanyVersion string

var (
	// CompanyVersionFreeTrail 免费试用版
	CompanyVersionFreeTrail CompanyVersion = "free_trail"
	// CompanyVersionProfessional 专业版
	CompanyVersionProfessional CompanyVersion = "professional"
	// CompanyVersionEnterprise 企业版
	CompanyVersionEnterprise CompanyVersion = "enterprise"
)

type ResourceQuota struct {
	DiskQuota     int64 `json:"disk_quota"`
	QAQuota       int   `json:"qa_quota"`
	AgentQuota    int   `json:"agent_quota"`
	EmployeeQuota int   `json:"employee_quota"`
	GraphQuota    int   `json:"graph_quota"`
	ArticleQuota  int   `json:"article_quota"`
}

type Company struct {
	gorm.Model

	// Name 公司名称
	Name string `gorm:"type:varchar(64);not null;uniqueIndex:udx_name" json:"name"`
	// Alias 公司别名
	Alias string `gorm:"type:varchar(64)" json:"alias"`
	// Description 公司描述
	Description string `gorm:"type:varchar(256)" json:"description"`
	// Logo 公司logo
	Logo string `gorm:"type:varchar(256)" json:"logo"`
	// Address 公司地址
	Address string `gorm:"type:varchar(256)" json:"address"`
	// Tel 公司电话
	Tel string `gorm:"type:varchar(256)" json:"tel"`
	// Email 公司邮箱
	Email string `gorm:"type:varchar(256)" json:"email"`
	// Website 公司网址
	Website string `gorm:"type:varchar(256)" json:"website"`
	// CompanyStatus 公司认证状态
	CompanyStatus CompanyStatus `gorm:"type:varchar(32)" json:"company_status"`
	// Version 团队版本
	Version CompanyVersion `gorm:"type:varchar(63);column:version;default:'free_trail'" json:"version"`
	// quota 资源配额
	Quota *ResourceQuota `gorm:"type:json;column:quota" json:"quota"`
	// UserID 创建者ID
	UserID uint `gorm:"column:user_id;type:bigint;default:0;comment:创建者ID"`
}

// TableName 表名
func (Company) TableName() string { return TableNameCompany }

// Value 实现 driver.Valuer 接口
func (r *ResourceQuota) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ResourceQuota to JSON: %w", err)
	}
	return string(jsonData), nil
}

func (r *ResourceQuota) Scan(src any) error {
	if src == nil {
		*r = ResourceQuota{}
		return nil
	}

	var sourceBytes []byte
	switch s := src.(type) {
	case string:
		sourceBytes = []byte(s)
	case []byte:
		sourceBytes = s
	default:
		return fmt.Errorf("unsupported Scan source type %T for ResourceQuota", src)
	}

	if len(sourceBytes) == 0 {
		*r = ResourceQuota{}
		return nil
	}

	err := json.Unmarshal(sourceBytes, r)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON to ResourceQuota: %w", err)
	}
	return nil
}

// CompanyStatus 公司认证状态
type CompanyStatus string

const (
	CompanyStatusPassed  CompanyStatus = "passed"
	CompanyStatusPending CompanyStatus = "pending"
	CompanyStatusFialed  CompanyStatus = "failed"
)

// CompanyInvitation 公司邀请表
type CompanyInvitation struct {
	gorm.Model

	// Issuer 签发公司
	Issuer string `gorm:"type:varchar(30);not null;index" json:"issuer"`
	// Count 邀请次数
	Count uint `gorm:"type:bigint;column:count;default:1" json:"count"`
	// Expired 过期时间
	Expired time.Time `gorm:"type:timestamptz;column:expired;not null" json:"expired"`
	// CompanyID 公司ID
	CompanyID uint `gorm:"type:bigint;column:company_id;not null;index" json:"company_id"`
	// Key 邀请码
	Key string `gorm:"type:varchar(255);not null;uniqueIndex" json:"key"`
	// AlreadyBind 是否已被使用
	AlreadyBind bool `gorm:"type:boolean;column:already_bind;default:false" json:"already_bind"`
	// InvitationRole 邀请角色
	InvitationRole SysRole `gorm:"type:varchar(50);column:role;not null" json:"role"`
	//PermSet
	PermSet *perm.Set `gorm:"type:text;column:perm_set" json:"perm_set"`
	//Uin
	Uin uint `gorm:"type:bigint;column:uin" json:"uin"`
	//DepartmentIDs
	DepartmentIDs types.UintArray `gorm:"type:varchar(255);column:department_ids;comment:'部门id列表'"`
}

// TableName 设置表名
func (CompanyInvitation) TableName() string {
	return TableNameCompanyInvitation
}

// IsExpire 是否过期
func (s CompanyInvitation) IsExpire() bool {
	return s.Expired.Before(time.Now())
}

type CompanyUpgradeApply struct {
	gorm.Model
	Name        string `gorm:"type:varchar(127);column:name" json:"name"`
	Phone       string `gorm:"type:varchar(127);column:phone" json:"phone"`
	CompanyName string `gorm:"type:varchar(255);column:company_name" json:"company_name"`
	Scale       string `gorm:"type:varchar(511);column:scale" json:"scale"`
	Industry    string `gorm:"type:varchar(1023);column:industry" json:"industry"`
	Note        string `gorm:"type:varchar(1023);column:note" json:"note"`
	//诉求
	Claim string `gorm:"type:varchar(1023);column:claim" json:"claim"`
	//表单类型
	Type FormType `gorm:"type:varchar(63);column:type" json:"type"`
}

type FormType string

var (
	// FormTypeContact 联系我们表单
	FormTypeContact FormType = "contact"
	// FormTypeUpgrade 审计版本表单
	FormTypeUpgrade FormType = "upgrade"
	// FormTypeDotpenContact 点评表单
	FormTypeDotpenContact FormType = "dotpen_contact"
)

func (CompanyUpgradeApply) TableName() string {
	return TableNameCompanyUpgradeApply
}
