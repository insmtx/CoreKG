package web

import (
	"context"
	"fmt"

	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"
)

type KeWebCrawlRule struct {
	ID       uint          `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`
	AppID    uint          `gorm:"column:app_id;type:bigint;not null;default:0;index:idx_crawl_rule_app" json:"app_id"`
	RuleType CrawlRuleType `gorm:"column:rule_type;type:varchar(32);not null;default:'include'" json:"rule_type"`
	Pattern  string        `gorm:"column:pattern;type:varchar(512);not null;default:''" json:"pattern"`
	Priority int           `gorm:"column:priority;type:int;not null;default:0" json:"priority"`
}

func (KeWebCrawlRule) TableName() string { return TableNameKeWebCrawlRule }

type CrawlRuleDao struct {
	BaseModel
}

func NewCrawlRuleDao() *CrawlRuleDao {
	return &CrawlRuleDao{}
}

func (dao *CrawlRuleDao) TableName() string {
	return TableNameKeWebCrawlRule
}

func (dao *CrawlRuleDao) WithTx(db *gorm.DB) *CrawlRuleDao {
	return &CrawlRuleDao{
		BaseModel: BaseModel{DBClient: db},
	}
}

func (dao *CrawlRuleDao) Insert(ctx context.Context, entity *KeWebCrawlRule) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Create(entity).Error; err != nil {
		return fmt.Errorf("[CrawlRuleDao] Insert fail, entity:%s, err: %v", logs.JSON(entity), err)
	}
	return nil
}

func (dao *CrawlRuleDao) GetByID(ctx context.Context, id uint) (*KeWebCrawlRule, error) {
	var entity KeWebCrawlRule
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("id = ?", id).Find(&entity).Error; err != nil {
		return nil, fmt.Errorf("[CrawlRuleDao] GetByID fail, id:%d, err: %v", id, err)
	}
	if entity.ID == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (dao *CrawlRuleDao) ListByAppID(ctx context.Context, appID uint) ([]*KeWebCrawlRule, error) {
	var entityList []*KeWebCrawlRule
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("app_id = ?", appID).Find(&entityList).Error; err != nil {
		return nil, fmt.Errorf("[CrawlRuleDao] ListByAppID fail, appID:%d, err: %v", appID, err)
	}
	return entityList, nil
}

func (dao *CrawlRuleDao) Update(ctx context.Context, entity *KeWebCrawlRule) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Where("id = ?", entity.ID).Updates(entity).Error; err != nil {
		return fmt.Errorf("[CrawlRuleDao] Update fail, id:%d, err: %v", entity.ID, err)
	}
	return nil
}

func (dao *CrawlRuleDao) Delete(ctx context.Context, id uint) error {
	db := dao.DB(ctx).Table(dao.TableName())
	if err := db.Unscoped().Where("id = ?", id).Delete(&KeWebCrawlRule{}).Error; err != nil {
		return fmt.Errorf("[CrawlRuleDao] Delete fail, id:%d, err: %v", id, err)
	}
	return nil
}
