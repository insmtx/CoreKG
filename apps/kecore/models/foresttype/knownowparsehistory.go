package foresttype

import "gorm.io/gorm"

type ContentType string

// KnownowParseHistory  define table to store content about docs algo parsed history
type KnownowParseHistory struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`

	ForestName string `gorm:"type:varchar(64);column:forest_name;not null;comment:forest name;" json:"forestName"`

	Path string `gorm:"type:varchar(255);column:path;not null;comment:source file relative path" json:"path"`

	//md5
	MD5 string `gorm:"type:varchar(255);column:md5;not null;comment:check whether source file was touched" json:"md5"`

	//文件解析
	Content string `gorm:"type:text;column:content;comment:parsed content" json:"content"`
	//思维导图
	MindMap string `gorm:"type:text;column:mind_map;comment:mind_map content" json:"mind_map"`
	//智能分析
	Analysis string `gorm:"type:text;column:analysis;comment:analysis content" json:"analysis"`
}

func (KnownowParseHistory) TableName() string {
	return TableNameKnownowParseHistory
}
