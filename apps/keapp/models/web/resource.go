package web

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"
)

type KeWebResource struct {
	gorm.Model
	AppID        uint            `gorm:"column:app_id;type:bigint;not null;default:0;index:idx_web_res_app" json:"app_id"`
	CompanyID    uint            `gorm:"column:company_id;type:bigint;not null;default:0;index:idx_web_res_company" json:"company_id"`
	SourceURL    string          `gorm:"column:source_url;type:varchar(2048);not null;default:''" json:"source_url"`
	Title        string          `gorm:"column:title;type:varchar(512);not null;default:''" json:"title"`
	ResourceType string          `gorm:"column:resource_type;type:varchar(32);not null;default:'web'" json:"resource_type"`
	ContentHash  string          `gorm:"column:content_hash;type:varchar(64);not null;default:'';index:idx_web_res_hash" json:"content_hash"`
	ETag         string          `gorm:"column:etag;type:varchar(256);not null;default:''" json:"etag"`
	LastModified string          `gorm:"column:last_modified;type:varchar(64);not null;default:''" json:"last_modified"`
	Manifest     json.RawMessage `gorm:"column:manifest;type:json" json:"manifest"`
	IndexStatus  IndexStatus     `gorm:"column:index_status;type:varchar(32);not null;default:'pending';index:idx_web_res_status" json:"index_status"`
	IndexedAt    *time.Time      `gorm:"column:indexed_at;type:timestamptz" json:"indexed_at"`
	CrawlCount   int             `gorm:"column:crawl_count;type:int;not null;default:0" json:"crawl_count"`
	LastCrawlAt  *time.Time      `gorm:"column:last_crawl_at;type:timestamptz" json:"last_crawl_at"`
	CrawlError   string          `gorm:"column:crawl_error;type:text" json:"crawl_error"`
	Metadata     json.RawMessage `gorm:"column:metadata;type:json" json:"metadata"`
	ForestFileID *uint           `gorm:"column:forest_file_id;type:bigint" json:"forest_file_id"`
	RawContent   string          `gorm:"column:raw_content;type:text" json:"raw_content"`
}

func (KeWebResource) TableName() string { return TableNameKeWebResource }

type WebResourceDao struct {
	BaseModel
}

func NewWebResourceDao() *WebResourceDao {
	return &WebResourceDao{}
}

func (dao *WebResourceDao) TableName() string {
	return TableNameKeWebResource
}

func (dao *WebResourceDao) WithTx(db *gorm.DB) *WebResourceDao {
	return &WebResourceDao{
		BaseModel: BaseModel{DBClient: db},
	}
}

func (dao *WebResourceDao) Insert(ctx context.Context, entity *KeWebResource) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Create(entity).Error; err != nil {
		return fmt.Errorf("[WebResourceDao] Insert fail, entity:%s, err: %v", logs.JSON(entity), err)
	}
	return nil
}

func (dao *WebResourceDao) GetByID(ctx context.Context, id uint) (*KeWebResource, error) {
	var entity KeWebResource
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("id = ?", id).Find(&entity).Error; err != nil {
		return nil, fmt.Errorf("[WebResourceDao] GetByID fail, id:%d, err: %v", id, err)
	}
	if entity.ID == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (dao *WebResourceDao) GetByURL(ctx context.Context, appID uint, url string) (*KeWebResource, error) {
	var entity KeWebResource
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("app_id = ? AND source_url = ?", appID, url).Find(&entity).Error; err != nil {
		return nil, fmt.Errorf("[WebResourceDao] GetByURL fail, appID:%d, url:%s, err: %v", appID, url, err)
	}
	if entity.ID == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (dao *WebResourceDao) ListByAppID(ctx context.Context, appID uint, limit int, offset int) ([]*KeWebResource, int64, error) {
	db := dao.DB(ctx).Model(&KeWebResource{}).Table(dao.TableName())
	db = db.Where("app_id = ?", appID)

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("[WebResourceDao] ListByAppID count fail, appID:%d, err: %v", appID, err)
	}
	if limit > 0 {
		db.Limit(limit)
	}
	if offset > 0 {
		db.Offset(offset)
	}
	var entityList []*KeWebResource
	if err := db.Find(&entityList).Error; err != nil {
		return nil, 0, fmt.Errorf("[WebResourceDao] ListByAppID find fail, appID:%d, err: %v", appID, err)
	}
	return entityList, count, nil
}

func (dao *WebResourceDao) Update(ctx context.Context, entity *KeWebResource) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("id = ?", entity.ID).Updates(entity).Error; err != nil {
		return fmt.Errorf("[WebResourceDao] Update fail, id:%d, err: %v", entity.ID, err)
	}
	return nil
}

func (dao *WebResourceDao) SoftDelete(ctx context.Context, id uint) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("id = ?", id).Delete(&KeWebResource{}).Error; err != nil {
		return fmt.Errorf("[WebResourceDao] SoftDelete fail, id:%d, err: %v", id, err)
	}
	return nil
}
