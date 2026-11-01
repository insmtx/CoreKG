package foresttype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/insmtx/corekg/pkgs/types"
	"gorm.io/gorm"
)

type DataSourceType string

const (
	DataSourceTypeDC DataSourceType = "DC"
)

type KnownowQASessionType string

const (
	KnownowQASessionTypeForest         KnownowQASessionType = "forest"
	KnownowQASessionTypeFile           KnownowQASessionType = "file"
	KnownowQASessionTypeFileList       KnownowQASessionType = "file_list"
	KnownowQASessionTypeDirList        KnownowQASessionType = "dir_list"
	KnownowQASessionTypeExcelList      KnownowQASessionType = "excel_list"
	KnownowQASessionTypeExcelSheetList KnownowQASessionType = "excel_sheet_list"
)

type KnownowQASessionBaseType string

const (
	KnownowQASessionBaseTypeStandard KnownowQASessionBaseType = "standard"
	KnownowQASessionBaseTypeExcel    KnownowQASessionBaseType = "excel"
	KnownowQASessionBaseTypeDatabase KnownowQASessionBaseType = "database"
)

const DefaultSessionName = "New Chat"

type KnownowQASession struct {
	gorm.Model
	Uin              uint                     `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID        uint                     `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	Type             KnownowQASessionType     `gorm:"column:type;type:varchar(255);not null;comment:'类型'" json:"type"`
	BaseType         KnownowQASessionBaseType `gorm:"column:base_type;type:varchar(32);default:'standard';comment:'基础类型，standard：标准, data_excel：Excel, data_mysql：MySQL'" json:"mode"`
	FileID           uint                     `gorm:"type:int;column:file_id;not null;default:0;comment:file_id" json:"file_id"`
	FileIDList       types.UintArray          `gorm:"column:file_id_list;type:text;comment:'文件ID列表'" json:"file_id_list"`
	ForestIDList     types.UintArray          `gorm:"column:forest_id_list;type:text;comment:'森林ID列表'" json:"forest_id_list"`
	ExcelIDList      types.UintArray          `gorm:"column:excel_id_list;type:text;comment:'excelIDList'" json:"excel_id_list"`
	ExcelSheetIDList types.UintArray          `gorm:"column:excel_sheet_id_list;type:text;comment:'excelSheetIDList'" json:"excel_sheet_id_list"`
	Name             string                   `gorm:"column:name;type:varchar(255);default:'New Chat';comment:'名称'" json:"name"`
	EsIndex          string                   `gorm:"column:es_index;type:varchar(255);default:'ke_0';comment:'es索引'" json:"es_index"`
	LLMModelID       uint                     `gorm:"column:llm_model_id;type:bigint;default:0;comment:'llm模型id'" json:"llm_model_id"`
}

func (KnownowQASession) TableName() string {
	return TableNameKnownowQASession
}

// KnownowForestQA 知识森林问答
type KnownowForestQA struct {
	gorm.Model

	SessionID uint `gorm:"type:int;column:session_id;not null;comment:session_id" json:"session_id"`
	Uin       uint `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID uint `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	// 聊天信息
	// Question 问题
	Question string `gorm:"column:question;type:text;comment:'问题'" json:"question"`
	// Answer 回答
	Answer string `gorm:"column:answer;type:text;comment:'回答'" json:"answer"`
	// 推理 reasoning
	Reasoning    string            `gorm:"column:reasoning;type:text;comment:'推理'" json:"reasoning"`
	MindGraph    string            `gorm:"column:mind_graph;type:text;comment:'思维导图'" json:"mind_graph"`
	ImageContent string            `gorm:"column:content;type:text;comment:'image用到的content'" json:"content"`
	ImageUrlList types.StringArray `gorm:"column:image_url_list;type:text;comment:'图片列表'" json:"image_url_list"`
	// Status 状态
	Status QAStatus `gorm:"column:status;type:varchar(8);not null;default:'pending';comment:'状态'" json:"status"`

	QueryReferenceList ChatReferenceList `gorm:"column:query_reference_list;type:text;comment:'引用内容列表';serializer:json" json:"query_reference_list"`
	ChatReferenceList  ChatReferenceList `gorm:"column:chat_reference_list;type:text;comment:'引用内容列表';serializer:json" json:"chat_reference_list"`
}

// TableName 表名
func (KnownowForestQA) TableName() string {
	return TableNameKnownowForestQA
}

// ChatReference 聊天的引用内容
type ChatReference struct {
	FileID   uint   `json:"file_id"`
	Filename string `json:"filename"`
	ForestID uint   `json:"forest_id"`
	// DataSourceType 数据源类型 DC
	DataSourceType DataSourceType `json:"data_source"`
	// ChatReferenceChunk `json:",inline"`
	ChunkList ChatReferenceChunkList `json:"chunk_list"`
}

// ChatReferenceChunk 引用内容
type ChatReferenceChunk struct {
	ChunkID  string `json:"chunk_id"`
	Sequence int    `json:"sequence"`
	Content  string `json:"content"`
	ImageURL string `json:"image_url"`
}

// ChatReferenceChunkList 引用内容列表
type ChatReferenceChunkList []ChatReferenceChunk

// ChatReferenceList 引用内容列表
type ChatReferenceList []*ChatReference

func (ep ChatReferenceList) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *ChatReferenceList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for ExamPosition")
	}
	return json.Unmarshal(bytes, ep)
}

func (crl ChatReferenceList) Len() int {
	return len(crl)
}

func (crl ChatReferenceList) Less(i, j int) bool {
	if len(crl[i].ChunkList) > len(crl[j].ChunkList) {
		return true
	}
	return crl[i].FileID < crl[j].FileID
}

func (crl ChatReferenceList) Swap(i, j int) {
	crl[i], crl[j] = crl[j], crl[i]
}

func (crcl ChatReferenceChunkList) Len() int {
	return len(crcl)
}

func (crcl ChatReferenceChunkList) Less(i, j int) bool {
	return crcl[i].Sequence < crcl[j].Sequence
}

func (crcl ChatReferenceChunkList) Swap(i, j int) {
	crcl[i], crcl[j] = crcl[j], crcl[i]
}

func (crcl *ChatReferenceChunkList) DeduplicateByContent() {
	seen := make(map[string]bool)
	writeIndex := 0

	for _, chunk := range *crcl {
		if !seen[chunk.Content] {
			seen[chunk.Content] = true
			(*crcl)[writeIndex] = chunk
			writeIndex++
		}
	}

	*crcl = (*crcl)[:writeIndex]
}
