package mysql

import (
	"time"

	"gorm.io/gorm"
)

type ExecStatus string

const (
	ExecStatusSuccess = "succ"
	ExecStatusFailed  = "fail"
)

// InitExecRecourd 初始化执行记录
type InitExecRecourd struct {
	gorm.Model
	ExecBatch string `gorm:"column:exec_batch;type:varchar(255);not null;comment:执行批次"`
	FileName  string `gorm:"column:file_name;type:varchar(255);not null;comment:文件名"`
	// FileHash     string     `gorm:"column:file_hash;type:varchar(255);not null;comment:文件hash"`
	StartTime    time.Time  `gorm:"column:start_time;type:timestamptz;not null;comment:开始时间"`
	EndTime      time.Time  `gorm:"column:end_time;type:timestamptz;not null;comment:结束时间"`
	ExecTime     float64    `gorm:"column:exec_time;type:float;not null;comment:执行时间"`
	ErrorMessage string     `gorm:"column:error_message;type:varchar(4096);not null;comment:错误信息"`
	FailLineAt   int        `gorm:"column:fail_line_at;type:int;not null;comment:失败行号"`
	ExecStatus   ExecStatus `gorm:"column:exec_status;type:varchar(10);not null;comment:执行状态"`
}

// TableName gorm table name
func (InitExecRecourd) TableName() string {
	return "yg_init_exec_recourd"
}

func ListAllSuccessExecRecourd(db *gorm.DB) ([]InitExecRecourd, error) {
	var records []InitExecRecourd
	if err := db.Where("exec_status = ?", ExecStatusSuccess).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func ListAllSuccessExecRecourdMap(db *gorm.DB) (map[string]InitExecRecourd, error) {
	list, err := ListAllSuccessExecRecourd(db)
	if err != nil {
		return nil, err
	}
	m := make(map[string]InitExecRecourd, len(list))
	for _, v := range list {
		m[v.FileName] = v
	}
	return m, nil
}

// LastExecRecourd 返回最近一次执行记录
func LastExecRecourd(db *gorm.DB, filename string) (*InitExecRecourd, error) {
	var rcd InitExecRecourd
	if err := db.Where("file_name = ?", filename).Order("exec_time desc").First(&rcd).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rcd, nil
}
