package chattype

import (
	"gorm.io/gorm"
)

// ChatChartCanvas chart图表画布表结构体
type ChatChartCanvas struct {
	gorm.Model
	SubjectID   uint               `gorm:"column:subject_id;type:bigint;not null;default 0;comment:主体 id"`
	SubjectType SessionSubjectType `gorm:"column:subject_type;type:varchar(32);not null;;comment:主体类型"`
	Content     string             `gorm:"column:content;type:text;not null;;comment:画布内容(json)"`
	CompanyID   uint               `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司ID"`
	Uin         uint               `gorm:"column:uin;type:bigint;not null;default 0;comment:用户uin"`
}

type ChatChartCanvasList []ChatChartCanvas

func (ChatChartCanvas) TableName() string {
	return TableNameChatChartCanvas
}

func (l ChatChartCanvasList) ToMap() map[uint]ChatChartCanvas {
	m := make(map[uint]ChatChartCanvas)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
