package accounttype

import (
	"gorm.io/gorm"
)

// AccountDepartment 部门表表结构体
type AccountDepartment struct {
	gorm.Model
	Name      string `gorm:"column:name;type:varchar(100);not null;;comment:部门名称，全局唯一"`
	ParentID  uint   `gorm:"column:parent_id;type:bigint;not null;default 0;comment:父部门ID，0表示顶层部门"`
	Sort      uint   `gorm:"column:sort;type:bigint;not null;default 0;comment:同级排序"`
	CompanyID uint   `gorm:"column:company_id;type:bigint;not null;default 0;comment:组织ID"`
}

type AccountDepartmentList []AccountDepartment

func (AccountDepartment) TableName() string {
	return TableNameAccountDepartment
}

func (l AccountDepartmentList) ToMap() map[uint]AccountDepartment {
	m := make(map[uint]AccountDepartment)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
