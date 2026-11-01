package foresttype

import "gorm.io/gorm"

type ParseStatusType string

var (
	ParseStatusRunning ParseStatusType = "running"
	ParseStatusDone    ParseStatusType = "over"
	ParseStatusFailed  ParseStatusType = "failed"
	ParseStatusFree    ParseStatusType = "free"
)

// KnownowFileParse docs parse for customer's storage
type KnownowFileParse struct {
	gorm.Model

	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';uniqueIndex:id_forest_path_md5_index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`

	ForestName string `gorm:"type:varchar(64);column:forest_name;not null;comment:forest name;uniqueIndex:id_forest_path_md5_index" json:"forestName"`

	Path string `gorm:"type:varchar(255);column:path;not null;comment:source file relative path;uniqueIndex:id_forest_path_md5_index" json:"path"`

	//文件一致性md5
	MD5 string `gorm:"type:varchar(255);column:md5;not null;comment:check whether source file was touched;uniqueIndex:id_forest_path_md5_index" json:"md5"`

	//文件解析
	Content string `gorm:"type:text;column:content;comment:parsed content" json:"content"`
	//思维导图
	MindMap string `gorm:"type:text;column:mind_map;comment:mind_map content" json:"mind_map"`
	//智能分析
	Analysis string `gorm:"type:text;column:analysis;comment:analysis content" json:"analysis"`

	//解析状态
	Status ParseStatusType `gorm:"type:varchar(30);column:status;not null;comment:parse status of forest" json:"status"`
}

func (KnownowFileParse) TableName() string {
	return TableNameKnownowFileParse
}
