package foresttype

import (
	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

type CmdString string

var (
	// CmdAbbreviation 缩写
	CmdAbbreviation CmdString = "abbreviation"
	// CmdExpansion 扩写
	CmdExpansion CmdString = "expansion"
	// CmdEmbellishment 润色
	CmdEmbellishment CmdString = "embellishment"
	// CmdProofreading 校阅
	CmdProofreading CmdString = "proofreading"
	// CmdContinuation 续写
	CmdContinuation CmdString = "continuation"
	// CmdOutlineGeneration 智能生成大纲
	CmdOutlineGeneration CmdString = "outline_generation"
	// CmdStructureOptimization 一键优化结构
	CmdStructureOptimization CmdString = "structure_optimization"
	// CmdProfessionalExpression 增强专业表达
	CmdProfessionalExpression CmdString = "professional_expression"
	// CmdKeyPointsExtraction 提取关键要点
	CmdKeyPointsExtraction CmdString = "key_points_extraction"
)

type TemplateType string

const (
	TemplateTypeSystem TemplateType = "system"
	TemplateTypeUser   TemplateType = "user"
)

type SourceType string

const (
	SourceTypeManual   SourceType = "manual"
	SourceTypeArticle  SourceType = "article"
	SourceTypeTemplate SourceType = "template"
)

type ArticleType string

const (
	ArticleTypeArticle        ArticleType = "article"
	ArticleTypeTemplateSystem ArticleType = "template_system"
	ArticleTypeTemplateUser   ArticleType = "template_user"
)

// KeArticle 写作空间文章表
type KeArticle struct {
	gorm.Model
	Type        ArticleType     `gorm:"column:type;type:varchar(32);not null;default:article;comment:类型：article=普通文章，template_system=系统模板，template_user=用户模板" json:"type"`
	Title       string          `gorm:"column:title;type:varchar(255);not null;comment:文章标题" json:"title"`
	Description string          `gorm:"column:description;type:varchar(512);not null;default:'';comment:文章描述" json:"description"`
	Content     string          `gorm:"column:content;type:text;comment:文章详细内容" json:"content"`
	SourceType  SourceType      `gorm:"column:source_type;type:varchar(32);not null;default:manual;comment:记录来源：manual=手动创建，article=基于文章创建，template=基于模板创建" json:"source_type"`
	SourceID    uint            `gorm:"column:source_id;type:bigint;not null;default:0;comment:来源资源id，配合source_type使用" json:"source_id"`
	ForestIDs   types.UintArray `gorm:"column:forest_ids;type:varchar(255);comment:知识库id列表" json:"forest_ids"`
	PublicScope PublicScope     `gorm:"column:public_scope;type:varchar(63);default:company;comment:资源公开范围" json:"public_scope"`
	CompanyID   uint            `gorm:"column:company_id;type:bigint;default:0;comment:公司id" json:"company_id"`
	Uin         uint            `gorm:"column:uin;type:bigint;default:0;comment:uin" json:"uin"`
	AvatarUrl   string          `gorm:"column:avatar_url;type:varchar(511);not null;default:'';comment:文章头像" json:"avatar_url"`
}

type KeArticleList []KeArticle

// TableName 设置表名
func (KeArticle) TableName() string {
	return TableNameKeArticle
}

// KeArticleTemplate 写作空间文章模板表
// Deprecated: 已合并到 KeArticle，通过 Type 字段区分。仅供数据迁移参考。
type KeArticleTemplate struct {
	gorm.Model
	Name         string       `gorm:"column:name;type:varchar(255);comment:模板名" json:"name"`
	Description  string       `gorm:"column:description;type:varchar(511);comment:描述" json:"description"`
	TemplateType TemplateType `gorm:"column:template_type;type:varchar(32);not null;default:'';comment:模板类型：system=系统模板，user=用户模板" json:"template_type"`
	SourceType   SourceType   `gorm:"column:source_type;type:varchar(32);not null;default:'manual';comment:来源类型：manual=手动创建，article=基于文章创建" json:"source_type"`
	SourceID     uint         `gorm:"column:source_id;type:bigint;not null;default:0;comment:来源资源id，配合source_type使用" json:"source_id"`
	Content      string       `gorm:"column:content;type:text;comment:模板内容" json:"content"`
	CompanyID    uint         `gorm:"column:company_id;type:bigint;default:0;comment:公司id" json:"company_id"`
	Uin          uint         `gorm:"column:uin;type:bigint;default:0;comment:uin" json:"uin"`
}

type KeArticleTemplateList []KeArticleTemplate

func (KeArticleTemplate) TableName() string {
	return TableNameKeArticleTemplate
}

// KeArticleHistory 写作空间撰写历史
type KeArticleHistory struct {
	gorm.Model
	ArticleID uint      `gorm:"column:article_id;type:bigint;not null;default:0;comment:关联文章id（ke_article.id）" json:"article_id"`
	Cmd       CmdString `gorm:"column:cmd;type:varchar(127);comment:撰写命令" json:"cmd"`
	Content   string    `gorm:"column:content;type:text;comment:原始内容" json:"content"`
	Result    string    `gorm:"column:result;type:text;comment:撰写结果" json:"result"`
	CompanyID uint      `gorm:"column:company_id;type:bigint;default:0;comment:公司id" json:"company_id"`
	Uin       uint      `gorm:"column:uin;type:bigint;default:0;comment:uin" json:"uin"`
}

type KeArticleHistoryList []KeArticleHistory

func (KeArticleHistory) TableName() string {
	return TableNameKeArticleHistory
}
