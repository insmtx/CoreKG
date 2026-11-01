package foresttype

import (
	"time"

	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

// ForestTable 知识库数据表表结构体
type ForestTable struct {
	gorm.Model
	ColumnCount  uint       `gorm:"column:column_count;type:smallint;not null;default 0;comment:列数"`
	DBID         uint       `gorm:"column:db_id;type:bigint;not null;;comment:数据库ID"`
	DBInstanceID uint       `gorm:"column:db_instance_id;type:bigint;not null;;comment:数据库实例ID"`
	ForestID     uint       `gorm:"column:forest_id;type:bigint;not null;;comment:知识库ID"`
	RowCount     uint       `gorm:"column:row_count;type:bigint;not null;default 0;comment:行数"`
	Size         uint       `gorm:"column:size;type:bigint;not null;default 0;comment:数据表大小（Bytes）"`
	SyncedAt     time.Time  `gorm:"column:synced_at;type:timestamptz;not null;default CURRENT_TIMESTAMP;comment:同步时间"`
	TableMeta    string     `gorm:"column:table_meta;type:text;;;comment:表元数据（如字段结构的JSON描述）"`
	Tablename    string     `gorm:"column:table_name;type:varchar(255);not null;;comment:表名"`
	Uin          uint       `gorm:"column:uin;type:bigint;not null;default 0;comment:用户uin"`
	Enable       types.Bool `gorm:"column:enable;type:smallint;not null;default 1;comment:是否启用"`
}

type ForestTableList []ForestTable

func (ForestTable) TableName() string {
	return TableNameKeForestTable
}

func (l ForestTableList) ToMap() map[uint]ForestTable {
	m := make(map[uint]ForestTable)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
