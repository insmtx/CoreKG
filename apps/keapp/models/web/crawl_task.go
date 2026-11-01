package web

import (
	"context"
	"fmt"
	"time"

	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"
)

type KeCrawlTask struct {
	ID           uint            `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`
	AppID        uint            `gorm:"column:app_id;type:bigint;not null;default:0;index:idx_crawl_task_app" json:"app_id"`
	ResourceID   *uint           `gorm:"column:resource_id;type:bigint" json:"resource_id"`
	SourceURL    string          `gorm:"column:source_url;type:varchar(2048);not null;default:''" json:"source_url"`
	TaskType     CrawlTaskType   `gorm:"column:task_type;type:varchar(32);not null;default:'full'" json:"task_type"`
	Status       CrawlTaskStatus `gorm:"column:status;type:varchar(32);not null;default:'pending';index:idx_crawl_task_status" json:"status"`
	ErrorMessage string          `gorm:"column:error_message;type:text" json:"error_message"`
	PagesCrawled int             `gorm:"column:pages_crawled;type:int;not null;default:0" json:"pages_crawled"`
	PagesTotal   int             `gorm:"column:pages_total;type:int;not null;default:0" json:"pages_total"`
	PagesNew     int             `gorm:"column:pages_new;type:int;not null;default:0" json:"pages_new"`
	PagesUpdated int             `gorm:"column:pages_updated;type:int;not null;default:0" json:"pages_updated"`
	PagesSkipped int             `gorm:"column:pages_skipped;type:int;not null;default:0" json:"pages_skipped"`
	StartedAt    *time.Time      `gorm:"column:started_at;type:timestamptz" json:"started_at"`
	FinishedAt   *time.Time      `gorm:"column:finished_at;type:timestamptz" json:"finished_at"`
	CreatedBy    uint            `gorm:"column:created_by;type:bigint;not null;default:0" json:"created_by"`
	CreatedAt    time.Time       `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
	UpdatedAt    time.Time       `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP(3)" json:"updated_at"`
}

func (KeCrawlTask) TableName() string { return TableNameKeCrawlTask }

type CrawlTaskDao struct {
	BaseModel
}

func NewCrawlTaskDao() *CrawlTaskDao {
	return &CrawlTaskDao{}
}

func (dao *CrawlTaskDao) TableName() string {
	return TableNameKeCrawlTask
}

func (dao *CrawlTaskDao) WithTx(db *gorm.DB) *CrawlTaskDao {
	return &CrawlTaskDao{
		BaseModel: BaseModel{DBClient: db},
	}
}

func (dao *CrawlTaskDao) Insert(ctx context.Context, entity *KeCrawlTask) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Create(entity).Error; err != nil {
		return fmt.Errorf("[CrawlTaskDao] Insert fail, entity:%s, err: %v", logs.JSON(entity), err)
	}
	return nil
}

func (dao *CrawlTaskDao) GetByID(ctx context.Context, id uint) (*KeCrawlTask, error) {
	var entity KeCrawlTask
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("id = ?", id).Find(&entity).Error; err != nil {
		return nil, fmt.Errorf("[CrawlTaskDao] GetByID fail, id:%d, err: %v", id, err)
	}
	if entity.ID == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (dao *CrawlTaskDao) UpdateStatus(ctx context.Context, id uint, status CrawlTaskStatus, errMsg string) error {
	db := dao.DB(ctx).Table(dao.TableName())
	updateMap := map[string]interface{}{
		"status":        status,
		"error_message": errMsg,
	}
	if err := db.Where("id = ?", id).Updates(updateMap).Error; err != nil {
		return fmt.Errorf("[CrawlTaskDao] UpdateStatus fail, id:%d, status:%s, err: %v", id, status, err)
	}
	return nil
}

func (dao *CrawlTaskDao) UpdateProgress(ctx context.Context, id uint, crawled, total, newPages, updated, skipped int) error {
	db := dao.DB(ctx).Table(dao.TableName())
	updateMap := map[string]interface{}{
		"pages_crawled": crawled,
		"pages_total":   total,
		"pages_new":     newPages,
		"pages_updated": updated,
		"pages_skipped": skipped,
	}
	if err := db.Where("id = ?", id).Updates(updateMap).Error; err != nil {
		return fmt.Errorf("[CrawlTaskDao] UpdateProgress fail, id:%d, err: %v", id, err)
	}
	return nil
}

func (dao *CrawlTaskDao) CancelTask(ctx context.Context, id uint) error {
	db := dao.DB(ctx).Table(dao.TableName())
	updateMap := map[string]interface{}{
		"status": CrawlTaskCancelled,
	}
	if err := db.Where("id = ?", id).Updates(updateMap).Error; err != nil {
		return fmt.Errorf("[CrawlTaskDao] CancelTask fail, id:%d, err: %v", id, err)
	}
	return nil
}

func (dao *CrawlTaskDao) ListByAppID(ctx context.Context, appID uint, limit int, offset int) ([]*KeCrawlTask, error) {
	db := dao.DB(ctx).Table(dao.TableName())
	db = db.Where("app_id = ?", appID).Order("created_at DESC")
	if limit > 0 {
		db.Limit(limit)
	}
	if offset > 0 {
		db.Offset(offset)
	}
	var entityList []*KeCrawlTask
	if err := db.Find(&entityList).Error; err != nil {
		return nil, fmt.Errorf("[CrawlTaskDao] ListByAppID fail, appID:%d, err: %v", appID, err)
	}
	return entityList, nil
}

func (dao *CrawlTaskDao) GetPendingAndRunning(ctx context.Context) ([]*KeCrawlTask, error) {
	var entityList []*KeCrawlTask
	db := dao.DB(ctx).Table(dao.TableName())
	statuses := []CrawlTaskStatus{CrawlTaskPending, CrawlTaskRunning}
	if err := db.Where("status IN ?", statuses).Find(&entityList).Error; err != nil {
		return nil, fmt.Errorf("[CrawlTaskDao] GetPendingAndRunning fail, err: %v", err)
	}
	return entityList, nil
}
