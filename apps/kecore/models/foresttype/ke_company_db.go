package foresttype

import (
	"gorm.io/gorm"
)

// KeCompanyDB 公司数据库表结构体
type KeCompanyDB struct {
	gorm.Model
	CompanyID    uint   `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司ID"`
	DBInstanceID uint   `gorm:"column:db_instance_id;type:bigint;not null;default 0;comment:数据库实例ID"`
	DBName       string `gorm:"column:db_name;type:varchar(255);not null;;comment:数据库名"`
}

type KeCompanyDBList []KeCompanyDB

func (KeCompanyDB) TableName() string {
	return TableNameKeCompanyDb
}

func (l KeCompanyDBList) ToMap() map[uint]KeCompanyDB {
	m := make(map[uint]KeCompanyDB)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
