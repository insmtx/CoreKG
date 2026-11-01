package accounttype

import (
	"gorm.io/gorm"
)

// AccountRelEmployeeDepartment 部门员工关联表表结构体
type AccountRelEmployeeDepartment struct {
	gorm.Model
	Uin          uint `gorm:"column:uin;type:bigint;not null;;comment:uin"`
	DepartmentID uint `gorm:"column:department_id;type:bigint;not null;;comment:部门ID"`
	IsPrimary    int8 `gorm:"column:is_primary;type:smallint;not null;default -1;comment:是否为主部门 (1:是, -1:否)"`
	EmployeeID   uint `gorm:"column:employee_id;type:bigint;not null;;comment:employee_id"`
	CompanyID    uint `gorm:"column:company_id;type:bigint;not null;;comment:组织ID"`
}

type AccountRelEmployeeDepartmentList []AccountRelEmployeeDepartment

func (AccountRelEmployeeDepartment) TableName() string {
	return TableNameAccountRelEmployeeDepartment
}

func (l AccountRelEmployeeDepartmentList) ToMap() map[uint]AccountRelEmployeeDepartment {
	m := make(map[uint]AccountRelEmployeeDepartment)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
