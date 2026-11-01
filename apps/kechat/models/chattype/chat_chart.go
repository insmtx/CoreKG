package chattype

import (
	"gorm.io/gorm"
)

type ChartType string

const (
	ChartTypeBar  = "bar"
	ChartTypeLine = "line"
	ChartTypePie  = "pie"
)

var ValidChartTypeMap = map[ChartType]struct{}{
	ChartTypeBar:  {},
	ChartTypeLine: {},
	ChartTypePie:  {},
}

// ChatChart chart图表表结构体
type ChatChart struct {
	gorm.Model
	RequestID    string             `gorm:"column:request_id;type:varchar(128);not null;;comment:请求ID"`
	QuestionID   string             `gorm:"column:question_id;type:varchar(128);not null;;comment:问题ID"`
	SessionID    uint               `gorm:"column:session_id;type:bigint;not null;default 0;comment:会话ID"`
	SubjectID    uint               `gorm:"column:subject_id;type:bigint;not null;default 0;comment:主体 id"`
	SubjectType  SessionSubjectType `gorm:"column:subject_type;type:varchar(32);not null;;comment:主体类型"`
	ChartType    ChartType          `gorm:"column:chart_type;type:varchar(32);not null;;comment:图表类型"`
	ChartContent string             `gorm:"column:chart_content;type:text;;;comment:内容"`
	CompanyID    uint               `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司ID"`
	Uin          uint               `gorm:"column:uin;type:bigint;not null;default 0;comment:用户uin"`
}

type ChatChartList []ChatChart

func (ChatChart) TableName() string {
	return TableNameChatChart
}

func (l ChatChartList) ToMap() map[uint]ChatChart {
	m := make(map[uint]ChatChart)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
