package chattype

import (
	"gorm.io/gorm"
)

// ChatQuestionDbDataset 数据库问答数据集表结构体
type ChatQuestionDbDataset struct {
	gorm.Model
	RequestID      string `gorm:"column:request_id;type:varchar(128);not null default '';comment:请求ID"`
	DatabaseType   string `gorm:"column:database_type;type:varchar(32);not null;;comment:数据库类型，mysql"`
	EchartsConfig  string `gorm:"column:echarts_config;type:text;;;comment:echarts图表配置"`
	EchartsDataset string `gorm:"column:echarts_dataset;type:text;;;comment:echarts数据集"`
	QueryResult    string `gorm:"column:query_result;type:text;;;comment:查询执行结果数据集"`
	QueryStatement string `gorm:"column:query_statement;type:text;;;comment:生成的查询语句（SQL/NoSQL/API等）"`
	QuestionID     string `gorm:"column:question_id;type:varchar(128);not null;;comment:问题ID"`
	SessionID      uint   `gorm:"column:session_id;type:bigint;not null;default 0;comment:会话ID"`
	TableList      string `gorm:"column:table_list;type:text;;;comment:相关表"`
}

type ChatQuestionDbDatasetList []ChatQuestionDbDataset

func (ChatQuestionDbDataset) TableName() string {
	return TableNameChatQuestionDbDataset
}

func (l ChatQuestionDbDatasetList) ToMap() map[uint]ChatQuestionDbDataset {
	m := make(map[uint]ChatQuestionDbDataset)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
