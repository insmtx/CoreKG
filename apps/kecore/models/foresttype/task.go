package foresttype

import (
	"time"

	"gorm.io/gorm"
)

type TaskType string

var (
	// TaskTypeParse 文件解析
	TaskTypeParse TaskType = "parse"
	// TaskTypeMindMap 思维导图
	TaskTypeMindMap TaskType = "mind_map"
	// TaskTypeAnalysis 智能分析
	TaskTypeAnalysis TaskType = "analysis"
	// TaskTypeKnowledge 单文件知识库
	TaskTypeKnowledge TaskType = "knowledge"
	// TaskTypeForestLib 知识森林知识库
	TaskTypeForestLib      TaskType = "forest_lib"
	TaskTypeResetForestLib TaskType = "reset_f_lib"
)

// KnownowTask 单文件任务
type KnownowTask struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	// ForestID 森林ID
	ForestID uint `gorm:"type:int;column:forest_id;not null;comment:forest id" json:"forest_id"`
	// FileID 文件ID
	FileID uint `gorm:"type:int;column:file_id;not null;comment:file id" json:"file_id"`

	// TaskType 任务类型 parse:文件解析 mind_map:思维导图 analysis:智能分析
	TaskType TaskType `gorm:"type:varchar(12);column:task_type;not null;comment:task type" json:"task_type"`
	// Priority 优先级
	Priority int `gorm:"type:smallint;column:priority;not null;comment:task priority" json:"priority"`
	// Path 文件相对路径
	Path string `gorm:"type:varchar(255);column:path;not null;comment:source file relative path" json:"path"`
	// TaskStatus 任务状态
	TaskStatus KnownowForestTaskStatus `gorm:"type:varchar(12);column:task_status;not null;comment:task status" json:"task_status"`
	// MachineID 机器ID 用于分布式任务
	MachineID string `gorm:"type:varchar(255);column:machine_id;comment:machine id" json:"machine_id"`
	// Redo 重试次数
	Redo int `gorm:"type:int;column:redo;not null;comment:redo times for parsing" json:"redo"`
	// ErrMsg 错误信息
	ErrMsg string `gorm:"type:varchar(255);column:err_msg;comment:error message" json:"err_msg"`
	// Comment 任务备注
	Comment string `gorm:"type:varchar(255);column:comment;comment:comment for parse task" json:"comment"`

	// StartAt 任务开始时间
	StartAt *time.Time `gorm:"type:timestamptz;column:start_at;comment:task start time" json:"start_at"`
	// EndAt 任务结束时间
	EndAt *time.Time `gorm:"type:timestamptz;column:end_at;comment:task end time" json:"end_at"`
	// Cost 耗时
	Cost int64 `gorm:"type:int;column:cost;comment:task cost" json:"cost"`
}

// TableName return table name
func (KnownowTask) TableName() string {
	return TableNameKnownowTask
}
