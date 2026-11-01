package foresttype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type ExcelHeaderMode string

const (
	ExcelHeaderModeRowTitle    ExcelHeaderMode = "row_title"
	ExcelHeaderModeColumnTitle ExcelHeaderMode = "column_title"
)

type ExcelSheetType string

const (
	ExcelSheetTypeNormal ExcelSheetType = "normal"
	ExcelSheetTypeSub    ExcelSheetType = "sub"
)

// ForestExcelSheet 知识库 excel sheet 信息表结构体
type ForestExcelSheet struct {
	gorm.Model
	ParentID        uint                 `gorm:"column:parent_id;type:bigint;not null;;comment:父 Sheet ID（0 表示顶层 Sheet）"`
	SheetType       ExcelSheetType       `gorm:"column:sheet_type;type:varchar(64);not null;comment:Sheet 类型：normal=普通 Sheet，sub=内嵌子 Sheet"`
	DataEndRowNum   int8                 `gorm:"column:data_end_row_num;type:smallint;not null;default 1;comment:数据结束行号"`
	DataStartRowNum int8                 `gorm:"column:data_start_row_num;type:smallint;not null;default 1;comment:数据开始行号"`
	ForestFileID    uint                 `gorm:"column:forest_file_id;type:bigint;not null;;comment:文件ID"`
	ForestID        uint                 `gorm:"column:forest_id;type:bigint;not null;;comment:知识库ID"`
	ForestTableID   uint                 `gorm:"column:forest_table_id;type:bigint;not null;;comment:关联数据表ID"`
	HeaderMode      ExcelHeaderMode      `gorm:"column:header_mode;type:varchar(16);not null;;comment:表头模式，row_title：行表头，column_title：列表头"`
	HeaderRowNum    int                  `gorm:"column:header_row_num;type:smallint;not null;default 1;comment:表头行号（从1开始）"`
	SheetMeta       ForestExcelSheetMeta `gorm:"column:sheet_meta;type:text;;;comment:Sheet元数据（如字段结构的JSON描述）"`
	SheetName       string               `gorm:"column:sheet_name;type:varchar(255);not null;;comment:Sheet名称"`
	TotalColumn     uint                 `gorm:"column:total_column;type:bigint;not null;default 0;comment:总列数（Excel数据列数）"`
	TotalRow        uint                 `gorm:"column:total_row;type:bigint;not null;default 0;comment:总行数（Excel数据行数）"`
}

type ForestExcelSheetList []ForestExcelSheet

func (ForestExcelSheet) TableName() string {
	return TableNameKeForestExcelSheet
}

func (l ForestExcelSheetList) ToMap() map[uint]ForestExcelSheet {
	m := make(map[uint]ForestExcelSheet)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}

type ForestExcelSheetMeta struct {
	ColumnMetadataList []ExcelColumnMetadata `json:"column_metadata_list"` // 列元数据
	// Summary 总结摘要
	Summary string `json:"summary"`
	// Remark 备注
	Remark string `json:"remark"`
}

type ExcelColumnMetadata struct {
	ExcelColumnName string `json:"excel_column_name"` // excel 列名称
	TableColumnName string `json:"table_column_name"` // 数据表列名称
	TableColumnType string `json:"table_column_type"` // 数据表列类型
}

func (d *ForestExcelSheetMeta) Scan(value interface{}) error {
	if d == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	return json.Unmarshal(bytes, d)
}

func (d ForestExcelSheetMeta) Value() (driver.Value, error) {
	return json.Marshal(d)
}
