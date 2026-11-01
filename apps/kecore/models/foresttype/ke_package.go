package foresttype

import (
	"gorm.io/gorm"
)

type PackageSourceType string

const (
	PackageSourceTypeSystem PackageSourceType = "system"
	PackageSourceTypeManual PackageSourceType = "manual"
)

type PackageEdition string

const (
	PackageEditionFreeTrail    PackageEdition = "free_trail"
	PackageEditionProfessional PackageEdition = "professional"
)

type PackageStatus string

const (
	PackageStatusDraft   PackageStatus = "draft"
	PackageStatusOnline  PackageStatus = "online"
	PackageStatusOffline PackageStatus = "offline"
)

type PackagePeriodType string

const (
	PackagePeriodTypeMonth    PackagePeriodType = "month"
	PackagePeriodTypeYear     PackagePeriodType = "year"
	PackagePeriodTypeLifetime PackagePeriodType = "lifetime"
)

type PackageLevel uint

const (
	PackageLevel1 PackageLevel = 1
	PackageLevel2 PackageLevel = 2
)

var PackageLevelDaysMap = map[PackageLevel]int64{
	PackageLevel1: 36500,
	PackageLevel2: 30,
}

// KePackage 会员套餐表结构体
type KePackage struct {
	gorm.Model
	Name          string            `gorm:"column:name;type:varchar(64);not null;;comment:套餐名称"`
	Description   string            `gorm:"column:description;type:varchar(512);;;comment:套餐描述"`
	Price         int64             `gorm:"column:price;type:bigint;not null;default 0;comment:价格（分）"`
	SalePrice     int64             `gorm:"column:sale_price;type:bigint;not null;default 0;comment:售价（分）"`
	Level         PackageLevel      `gorm:"column:level;type:smallint;not null;default 1;comment:套餐等级"`
	AgentQuota    int64             `gorm:"column:agent_quota;type:bigint;not null;default 0;comment:可创建智能体数量"`
	QaQuota       int64             `gorm:"column:qa_quota;type:bigint;not null;default 0;comment:每日问答次数"`
	DiskQuota     int64             `gorm:"column:disk_quota;type:bigint;not null;default 0;comment:磁盘配额（字节）"`
	EmployeeQuota int64             `gorm:"column:employee_quota;type:bigint;not null;default 0;comment:成员数量"`
	ArticleQuota  int64             `gorm:"column:article_quota;type:bigint;not null;default 0;comment:文章数量"`
	Edition       PackageEdition    `gorm:"column:edition;type:varchar(16);not null;default professional;comment:套餐类型，free_trail：免费版，professional：专业版"`
	SourceType    PackageSourceType `gorm:"column:source_type;type:varchar(16);not null;default manual;comment:套餐来源，system：系统内置，manual：运营添加"`
	PeriodType    PackagePeriodType `gorm:"column:period_type;type:varchar(16);not null;default month;comment:会员周期类型，month：月度会员，year：年度会员，lifetime:终身会员"`
	Extra         ExtraInfo         `gorm:"serializer:json;column:extra;type:text;;;comment:扩展信息（JSON格式）"`
	Status        PackageStatus     `gorm:"column:status;type:varchar(16);not null;default draft;comment:套餐状态：draft-草稿，online-上架，offline-下架"`
}

type ExtraInfo struct {
	// AdditionalNotes 辅助说明
	AdditionalNotes []string `json:"additional_notes"`
}

type KePackageList []KePackage

func (KePackage) TableName() string {
	return TableNameKePackage
}

func (l KePackageList) ToMap() map[uint]KePackage {
	m := make(map[uint]KePackage)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
