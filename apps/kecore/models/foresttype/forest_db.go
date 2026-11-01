package foresttype

import (
	"time"

	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

// ForestDB 知识库数据库表结构体
type ForestDB struct {
	gorm.Model
	CompanyID    uint         `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	DBInstanceID uint         `gorm:"column:db_instance_id;type:bigint;not null;default 0;comment:数据库实例ID"`
	DBMeta       ForestDBMeta `gorm:"serializer:json;column:db_meta;type:text;not null;;comment:数据库元数据，字符集等"`
	DBName       string       `gorm:"column:db_name;type:varchar(255);not null;;comment:数据库名"`
	ForestID     uint         `gorm:"column:forest_id;type:bigint;not null;default 0;comment:知识库ID"`
	Size         uint         `gorm:"column:size;type:bigint;not null;default 0;comment:数据库大小（Bytes）"`
	RowCount     uint         `gorm:"column:row_count;type:bigint;not null;default 0;comment:行数"`
	SyncedAt     time.Time    `gorm:"column:synced_at;type:timestamptz;not null;default CURRENT_TIMESTAMP;comment:同步时间"`
	Uin          uint         `gorm:"column:uin;type:bigint;not null;default 0;comment:用户uin"`
	Enable       types.Bool   `gorm:"type:smallint;column:enable;default:1;comment:是否启用" json:"enable"`
}

type ForestDBMeta struct {
	Mysql ForestDBMysqlMeta `json:"mysql"`
}

type ForestDBMysqlMeta struct {
	Charset string `json:"charset"`
	Collate string `json:"collate"`
}

type ForestDBList []ForestDB

func (ForestDB) TableName() string {
	return TableNameKeForestDB
}

func (l ForestDBList) ToMap() map[uint]ForestDB {
	m := make(map[uint]ForestDB)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
