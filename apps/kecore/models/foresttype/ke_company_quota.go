package foresttype

import (
	"time"

	"gorm.io/gorm"
)

type CompanyQuotaSourceType string

const (
	CompanyQuotaSourceTypeOrder  CompanyQuotaSourceType = "order"
	CompanyQuotaSourceTypeManual CompanyQuotaSourceType = "manual"
)

// KeCompanyQuota 公司配额表结构体
type KeCompanyQuota struct {
	gorm.Model
	CompanyID     uint                   `gorm:"column:company_id;type:bigint;not null;;comment:公司ID"`
	SourceType    CompanyQuotaSourceType `gorm:"column:source_type;type:varchar(16);not null;;comment:配额来源：order-订单，manual-手动发放"`
	PackageLevel  PackageLevel           `gorm:"column:package_level;type:smallint;not null;default 0;comment:套餐等级"`
	OperatorID    uint                   `gorm:"column:operator_id;type:bigint;;;comment:操作人ID（手动发放时记录操作人）"`
	AgentQuota    int64                  `gorm:"column:agent_quota;type:bigint;not null;default 0;comment:可创建智能体数量"`
	QaQuota       int64                  `gorm:"column:qa_quota;type:bigint;not null;default 0;comment:每日问答次数"`
	DiskQuota     int64                  `gorm:"column:disk_quota;type:bigint;not null;default 0;comment:磁盘配额（字节）"`
	EmployeeQuota int64                  `gorm:"column:employee_quota;type:bigint;not null;default 0;comment:成员数量"`
	ArticleQuota  int64                  `gorm:"column:article_quota;type:bigint;not null;default 0;comment:文章数量"`
	EffectiveAt   *time.Time             `gorm:"column:effective_at;type:timestamptz;;;comment:生效时间"`
	ExpireAt      *time.Time             `gorm:"column:expire_at;type:timestamptz;;;comment:过期时间"`
}

type KeCompanyQuotaList []KeCompanyQuota

func (KeCompanyQuota) TableName() string {
	return TableNameKeCompanyQuota
}

func (l KeCompanyQuotaList) ToMap() map[uint]KeCompanyQuota {
	m := make(map[uint]KeCompanyQuota)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
