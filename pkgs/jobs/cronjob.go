package jobs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/insmtx/corekg/pkgs/global"
	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TableNameCronJobs       = global.CoreTableNamePrefix + "cron_jobs"
	TableNameCronJobRecords = global.CoreTableNamePrefix + "cron_job_records"
)

const (
	CronJobStatusRunning = "running"
	CronJobStatusSuccess = "success"
	CronJobStatusFailed  = "failed"
)

var ownRunnerName string

func init() {
	hostname, _ := os.Hostname()
	ownRunnerName = fmt.Sprintf("%v(%s)", os.Getpid(), hostname)
}

type JobFunc func(ctx context.Context, out *bytes.Buffer) error

// CronJob 定时任务
type CronJob struct {
	gorm.Model

	// Name 任务名称
	Name string `gorm:"column:name;size:255;not null;unique" json:"name"`

	// CronStr 定时表达式 * * * * * (分 时 日 月 周)
	CronStr string `gorm:"column:cron_str;size:32;not null" json:"cron_str"`
	// IsEnabled 是否启用
	IsEnabled types.Bool `gorm:"column:is_enabled;type:smallint;not null;default:1" json:"is_enabled"`

	// NextRunTime 下次运行时间
	NextRunTime time.Time `gorm:"column:next_run_time;type:timestamptz" json:"next_run_time"`

	// LastRunTime 上次运行时间
	LastRunTime time.Time `gorm:"column:last_run_time;type:timestamptz" json:"last_run_time"`
	// LastRunStatus 上次运行状态
	LastRunStatus string `gorm:"column:last_run_status;size:8" json:"last_run_status"`
	// LastRunError 上次运行错误
	LastRunError string `gorm:"column:last_run_error;type:text" json:"last_run_error"`
	// LastRunOutput 上次运行输出
	LastRunOutput string `gorm:"column:last_run_output;type:text" json:"last_run_output"`
	// LastCostSeconds 上次运行耗时
	LastCostSeconds float64 `gorm:"column:last_cost_seconds;type:decimal(10,2);not null" json:"last_cost_seconds"`
	// LastRunnerName 上次运行执行者名称
	LastRunnerName string `gorm:"column:last_runner_name;size:32" json:"last_runner_name"`

	// RunCount 运行次数
	RunCount uint `gorm:"column:run_count" json:"run_count"`
}

// TableName 表名
func (CronJob) TableName() string {
	return TableNameCronJobs
}

// CronJobRecord 定时任务记录
type CronJobRecord struct {
	gorm.Model

	// CronJobID 定时任务ID
	CronJobID uint `gorm:"column:cron_job_id;not null;uniqueIndex:idx_cjid_runtime" json:"cron_job_id"`
	// RunTime 运行时间
	RunTime time.Time `gorm:"column:run_time;type:timestamptz;not null;uniqueIndex:idx_cjid_runtime" json:"run_time"`
	// RunnerName 执行者名称
	RunnerName string `gorm:"column:runner_name;size:32;not null" json:"runner_name"`
	// StartAt 开始时间
	StartAt time.Time `gorm:"column:start_at;type:timestamptz;not null" json:"start_at"`
	// Status 状态
	Status string `gorm:"column:status;size:16;not null" json:"status"`
	// Error 错误
	Error string `gorm:"column:error;type:text" json:"error"`
	// Output 输出
	Output string `gorm:"column:output;type:text" json:"output"`
	// CostSeconds 耗时
	CostSeconds float64 `gorm:"column:cost_seconds;type:decimal(10,2);not null" json:"cost_seconds"`
}

// TableName 表名
func (CronJobRecord) TableName() string {
	return TableNameCronJobRecords
}

// RegisterCronJob 注册定时任务
func RegisterCronJob(name, cronStr string, jobFunc JobFunc) error {
	if stdCJMgr == nil {
		panic("cron job manager is not initialized")
	}
	cj := &CronJob{
		Name:        name,
		CronStr:     cronStr,
		NextRunTime: time.Now(),
		LastRunTime: time.Now(),
	}
	return stdCJMgr.Register(cj, jobFunc)
}

// insertOrUpdateCronJob 插入或更新定时任务
func insertOrUpdateCronJob(db *gorm.DB, cj *CronJob) error {
	return db.Table(TableNameCronJobs).Clauses(clause.OnConflict{
		DoUpdates: clause.AssignmentColumns([]string{"cron_str", "is_enabled", "next_run_time"}),
	}).Create(cj).Error
}

// getCronJob 获取定时任务
func getCronJob(db *gorm.DB, name string) (*CronJob, error) {
	var cj CronJob
	err := db.Table(TableNameCronJobs).Where("name = ?", name).First(&cj).Error
	if err != nil {
		return nil, err
	}
	return &cj, nil
}
