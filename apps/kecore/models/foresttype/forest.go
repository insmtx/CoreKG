package foresttype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/insmtx/corekg/apps/ketask/models/ragtask"
	"github.com/insmtx/corekg/pkgs/utils/dbutil"
	"github.com/ygpkg/yg-go/storage"
	"github.com/ygpkg/yg-go/types"
)

// KnownowForestTaskStatus 知识森林文件解析状态
type KnownowForestTaskStatus = string

const (
	TaskStatusPending     KnownowForestTaskStatus = "pending"
	TaskStatusRunning     KnownowForestTaskStatus = "running"
	TaskStatusSuccess     KnownowForestTaskStatus = "success"
	TaskStatusFail        KnownowForestTaskStatus = "fail"
	StatusTimeout         KnownowForestTaskStatus = "timeout"
	TaskStatusUnsupported KnownowForestTaskStatus = "unsupported"

	//sub status 解析子状态

	// TaskStatusParsing 解析中
	TaskStatusParsing KnownowForestTaskStatus = "parsing"

	// TaskStatusIndexing 索引中
	TaskStatusIndexing KnownowForestTaskStatus = "indexing"
)

type PreViewAbleStatus = string

const (
	PreViewAbleStatusAccept      PreViewAbleStatus = "accept"
	PreViewAbleStatusUnsupported PreViewAbleStatus = "unsupported"
)

// PublicScope 公开范围
type PublicScope string

const (
	// PublicScopePublic 全局可见，最小单位为公司,可见公司列表依赖public_scope子表
	PublicScopePublic PublicScope = "public"
	// PublicScopePrivate 私有，仅创建者可见
	PublicScopePrivate PublicScope = "private"
	// PublicScopeCompany 公司内可见
	PublicScopeCompany PublicScope = "company"
	// PublicScopeCustom 自定义范围
	PublicScopeCustom PublicScope = "custom"
)

type ForestType string

const (
	ForestTypeFile ForestType = "file"
	ForestTypeQA   ForestType = "qa"
	ForestTypeCAD  ForestType = "cad"
	ForestTypeData ForestType = "data" // 数据知识库
)

type ForestDataSourceType string

const (
	ForestDataSourceStandard ForestDataSourceType = "standard"
	ForestDataSourceExcel    ForestDataSourceType = "excel"
	ForestDataSourceDB       ForestDataSourceType = "db"
)

type ForestDataSourceSubtype string

const (
	ForestDataSourceSubtypeStandard ForestDataSourceSubtype = "standard"
	ForestDataSourceSubtypeExcel    ForestDataSourceSubtype = "excel"
	ForestDataSourceSubtypeMySQL    ForestDataSourceSubtype = "mysql"
)

var ForestDataSourceTypeMap = map[ForestDataSourceType]struct{}{
	ForestDataSourceStandard: {},
	ForestDataSourceExcel:    {},
	ForestDataSourceDB:       {},
}

var ForestDataSourceSubtypeMap = map[ForestDataSourceSubtype]struct{}{
	ForestDataSourceSubtypeStandard: {},
	ForestDataSourceSubtypeExcel:    {},
	ForestDataSourceSubtypeMySQL:    {},
}

type ForestModule string

const (
	ForestModuleForest   ForestModule = "forest"
	ForestModuleDatabase ForestModule = "database"
	ForestModuleTable    ForestModule = "table"
)

var ForestModuleTableMap = map[ForestModule]string{
	ForestModuleForest:   TableNameKnownowForest,
	ForestModuleDatabase: TableNameKeForestDB,
	ForestModuleTable:    TableNameKeForestTable,
}

var ForestModuleNameFieldMap = map[ForestModule]string{
	ForestModuleDatabase: "db_name",
	ForestModuleTable:    "table_name",
}

// KnownowForest 知识库
type KnownowForest struct {
	gorm.Model
	Uin               uint                    `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID         uint                    `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	Name              string                  `gorm:"type:varchar(255);column:name;not null;comment:知识森林名称" json:"name"`
	KnowledgeStatus   KnownowForestTaskStatus `gorm:"type:varchar(64);column:knowledge_status;not null;default:pending;comment:知识库状态" json:"knowledge_status"`
	AvatarUrl         string                  `gorm:"type:varchar(255);column:avatar_url;not null;default:'';comment:知识森林头像" json:"avatar_url"`
	Description       string                  `gorm:"type:varchar(255);column:description;not null;default:'';comment:知识森林描述" json:"description"`
	ForestType        ForestType              `gorm:"type:varchar(64);column:forest_type;not null;default:'file';comment:知识森林类型" json:"forest_type"`
	DataSourceType    ForestDataSourceType    `gorm:"type:varchar(64);column:data_source_type;not null;default:'';comment:数据源类型" json:"data_source_type"`
	DataSourceSubType ForestDataSourceSubtype `gorm:"type:varchar(64);column:data_source_subtype;not null;default:'';comment:数据源子类型" json:"data_source_subtype"`
	// Config pdf2mardown配置
	// makrkdown trunk -> 语义+关系
	ConfigID uint `gorm:"type:bigint;column:config_id;not null;default:0;comment:配置ID" json:"config_id"`
	// PublicScope 公开范围
	PublicScope PublicScope `gorm:"column:public_scope;type:varchar(32);not null;default:'private';comment:公开范围" json:"public_scope"`
	// ManagerIDs 管理员ID
	ManagerIDs types.UintArray `gorm:"column:manager_ids;type:varchar(256);comment:管理员ID" json:"manager_ids"`
	// Count 资源计数
	Count int `gorm:"column:count;default:0;comment:知识库资源计数" json:"count"`
	// DiskStorage 磁盘占用
	DiskStorage string `gorm:"column:disk_storage;type:varchar(255);comment:知识库磁盘占用" json:"disk_storage"`

	// GraphStatus 知识图谱状态
	GraphStatus GraphStatus `gorm:"type:varchar(64);column:graph_status;not null;default:uncreated;comment:知识图谱状态" json:"graph_status"`
}

type KnownowForestList []KnownowForest

func (l KnownowForestList) ToMap() map[uint]KnownowForest {
	m := make(map[uint]KnownowForest)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}

func (KnownowForest) TableName() string {
	return TableNameKnownowForest
}

// EsIndex es索引
func (kf *KnownowForest) EsIndex() string {
	return fmt.Sprintf("ke_%d", kf.ConfigID)
}

const (
	PriviewExtXlsx string = ".xlsx"
	PriviewExtXls  string = ".xls"
)

// KnownowForestFile 知识森林文件
type KnownowForestFile struct {
	gorm.Model
	Uin             uint                    `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID       uint                    `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	ForestID        uint                    `gorm:"type:bigint;column:forest_id;not null;comment:知识森林id" json:"forest_id"`
	CoreFileID      uint                    `gorm:"type:bigint;column:file_id;not null;comment:文件id" json:"file_id"`
	PriviewFileID   uint                    `gorm:"type:bigint;column:priview_file_id;not null;comment:预览文件id" json:"priview_file_id"`
	PriviewExt      string                  `gorm:"type:varchar(64);column:priview_ext;not null;default:'';comment:预览文件后缀" json:"priview_ext"`
	IsDir           types.Bool              `gorm:"column:is_dir;type:smallint;not null;default:-1" json:"is_dir"`
	ParentID        uint                    `gorm:"type:int;column:parent_id;not null;comment:知识森林id" json:"parent_id"`
	Name            string                  `gorm:"type:varchar(255);column:name;not null;comment:名称" json:"name"`
	Size            int64                   `gorm:"type:bigint;column:size;not null;comment:文件大小" json:"size"`
	Ext             string                  `gorm:"type:varchar(64);column:ext;not null;comment:文件后缀" json:"ext"`
	ParentIDs       string                  `gorm:"type:varchar(255);column:parent_ids;not null;comment:parent_ids" json:"parent_ids"`
	Depth           int                     `gorm:"type:smallint;column:depth;not null;comment:深度" json:"depth"`
	ParseStatus     KnownowForestTaskStatus `gorm:"type:varchar(64);column:parse_status;not null;default:pending;comment:md解析状态" json:"parse_status"`
	MindmapStatus   KnownowForestTaskStatus `gorm:"type:varchar(64);column:mindmap_status;not null;default:pending;comment:思维导图状态" json:"mindmap_status"`
	AnalysisStatus  KnownowForestTaskStatus `gorm:"type:varchar(64);column:analysis_status;not null;default:pending;comment:智能分析状态" json:"analysis_status"`
	KnowledgeStatus KnownowForestTaskStatus `gorm:"type:varchar(64);column:knowledge_status;not null;default:pending;comment:知识库状态" json:"knowledge_status"`
	GraphStatus     KnownowForestTaskStatus `gorm:"type:varchar(64);column:graph_status;not null;default:pending;comment:知识图谱状态" json:"graph_status"`
	DescStatus      KnownowForestTaskStatus `gorm:"type:varchar(64);column:desc_status;not null;default:pending;comment:文件描述状态" json:"desc_status"`
	PreViewAble     PreViewAbleStatus       `gorm:"type:varchar(64);column:preview_able;default:accept;comment:文件是否可预览" json:"preview_able"`
	Enable          types.Bool              `gorm:"type:smallint;column:enable;default:1;comment:是否启用" json:"enable"`

	// Status 文件状态
	Status FileStatus `gorm:"column:status;type:varchar(15);default:'normal'" json:"status"`
	// Status 文件状态
	FileConfig FileConfig `gorm:"column:file_config;type:text;comment:'文件配置';serializer:json" json:"file_config"`
}

type KeForestFileList []KnownowForestFile

func (l KeForestFileList) ToMap() map[uint]KnownowForestFile {
	m := make(map[uint]KnownowForestFile)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}

// CalculateCompletionPercentage 根据 KnownowForestFile 的任务状态计算完成百分比
func (f *KnownowForestFile) CalculateCompletionPercentage() (string, string) {
	totalTasks := 2
	completedTasks := 0
	if f.ParseStatus == TaskStatusUnsupported {
		return TaskStatusUnsupported, "暂不支持" // 避免除以零
	}

	if f.ParseStatus == TaskStatusPending {
		return TaskStatusPending, "排队待处理" // pending
	}

	//解析失败
	if f.ParseStatus == TaskStatusFail ||
		// f.GraphStatus == TaskStatusFail ||
		//f.DescStatus == TaskStatusFail
		f.KnowledgeStatus == TaskStatusFail {
		//return TaskStatusFail, fmt.Sprintf("解析失败(%0.1f%%)", float32(completedTasks)/float32(totalTasks)*100)
		return TaskStatusFail, "解析失败" //fail
	}

	// 检查每个任务的状态，如果为 StatusSuccess 则认为是已完成
	if f.ParseStatus == TaskStatusSuccess {
		completedTasks++
	}
	if f.KnowledgeStatus == TaskStatusSuccess {
		completedTasks++
	}
	// if f.GraphStatus == TaskStatusSuccess {
	// 	completedTasks++
	// }

	//v2.2.0 don't include desc task when calculate file progress
	//if f.DescStatus == TaskStatusSuccess {
	//	completedTasks++
	//}

	switch completedTasks {
	case 0:
		return TaskStatusParsing, "资源解析中" //parsing
	case 1:
		return TaskStatusIndexing, "资源索引中" //indexing
	case totalTasks:
		return TaskStatusSuccess, "资源已就绪" //success
	default:
		return TaskStatusRunning, "进度计算失败" //unknown fail
	}
}

// CalculateProgress  根据 KnownowForestFile 的任务状态计算完成百分比
// 已完成、进行中（进度展示）、解析失败、暂不支持、待开始
func (f *KnownowForestFile) CalculateProgress() (string, string) {
	totalTasks := 3
	completedTasks := 0
	//暂不支持
	if f.ParseStatus == TaskStatusUnsupported {
		return TaskStatusUnsupported, "0"
	}

	//待开始
	if f.ParseStatus == TaskStatusPending {
		return TaskStatusPending, "0"
	}

	// 检查每个任务的状态，如果为 StatusSuccess 则认为是已完成
	if f.ParseStatus == TaskStatusSuccess {
		completedTasks++
	}

	if f.KnowledgeStatus == TaskStatusSuccess {
		completedTasks++
	}
	// if f.GraphStatus == TaskStatusSuccess {
	// 	completedTasks++
	// }
	if f.DescStatus == TaskStatusSuccess {
		completedTasks++
	}

	//进行中
	prog := fmt.Sprintf("%0.2f", float32(completedTasks)/float32(totalTasks))

	//解析失败
	if f.ParseStatus == TaskStatusFail ||
		f.KnowledgeStatus == TaskStatusFail ||
		// f.GraphStatus == TaskStatusFail ||
		f.DescStatus == TaskStatusFail {
		return TaskStatusFail, prog
	}

	if completedTasks == 0 {
		return TaskStatusPending, "0" // 避免除以零
	}

	//已完成
	if prog == "1.00" {
		return TaskStatusSuccess, prog
	}

	return TaskStatusRunning, prog
}

func (KnownowForestFile) TableName() string {
	return TableNameKnownowForestFile
}

// FileStatus 文件状态
type FileStatus string

const (
	// FileStatusPending 等待上传
	FileStatusPending FileStatus = "pending"
	// FileStatusNormal 正常
	FileStatusNormal FileStatus = "normal"
	// FileStatusFailed 上传失败
	FileStatusFailed FileStatus = "failed"
)

const (
	PurposeForestFile = "forest"   // 知识森林文件根路径
	PurposeForestAlgo = "algo-lke" // 知识森林算法根路径
	PurposeMdImage    = "md-img"   // markdown图片路径
)

type FileConfig struct {
	SplitConfig *ragtask.SplitConfig `json:"split_config"`
}

func (ep FileConfig) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *FileConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for FileConfig")
	}
	return json.Unmarshal(bytes, ep)
}

// minio不能用filepath.Join。不然文件名会变成4/5/6/7/8/9/10.pdf
// GetForestFilePath 获取知识森林存储路径
func (finfo *KnownowForestFile) GetForestFilePath() (path *string, err error) {
	return getCoreFilePathById(finfo.CoreFileID)
}

// minio不能用filepath.Join。不然文件名会变成4/5/6/7/8/9/10.pdf
// GetForestFilePath 获取知识森林存储路径
func (finfo *KnownowForestFile) GetForestPriviewFilePath() (path *string, err error) {
	return getCoreFilePathById(finfo.PriviewFileID)
}

// GetAlgoFilePath 获取算法存储文件夹路径
func (finfo *KnownowForestFile) GetAlgoFilePath() string {
	return fmt.Sprintf("%s/%d/%d", PurposeForestAlgo, finfo.Uin, finfo.ForestID)
}

func getCoreFilePathById(coreFileId uint) (path *string, err error) {
	coreFile, err := storage.GetFileByID(dbutil.Core(), coreFileId)
	if err != nil {
		return nil, err
	}
	if coreFile == nil {
		return nil, errors.New("core_file_not_found")
	}
	return &coreFile.StoragePath, nil
}

// ScopeType 范围类型
type ScopeType string

const (
	// ScopeTypePublic 公开
	ScopeTypePublic ScopeType = "public"

	// ScopeTypeCompany 公司
	ScopeTypeCompany ScopeType = "company"

	// ScopeTypeUser 用户
	ScopeTypeUser ScopeType = "user"

	// ScopeTypeDepartment 部门
	ScopeTypeDepartment ScopeType = "department"
)

type KnownowForestPublicScope struct {
	gorm.Model
	//  ForestID
	ForestID uint `gorm:"column:forest_id;type:int;not null;default:0;index:forest_id;comment:知识森林ID" json:"forest_id"`
	// ScopeType 公开范围类型
	ScopeType ScopeType `gorm:"column:scope_type;type:varchar(32);not null;default:'';index:scope_type;comment:公开范围类型" json:"scope_type"`
	//  ScopeID 公开范围ID
	ScopeID uint `gorm:"column:scope_id;type:int;not null;default:0;index:scope_id;comment:公开范围ID" json:"scope_id"`
}

func (KnownowForestPublicScope) TableName() string {
	return TableNameKnownowForestPublicScope
}
