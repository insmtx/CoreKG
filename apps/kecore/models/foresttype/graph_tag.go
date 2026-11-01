package foresttype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type TagType string

const (
	TagTypeNode TagType = "TAG"
	TagTypeEdge TagType = "EDGE"
)

type TagStatus string

const (
	TagStatusSynced TagStatus = "synced"
	TagStatusNot    TagStatus = "not_synced"
)

type GraphTag struct {
	gorm.Model
	Uin            uint       `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID      uint       `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	Description    string     `gorm:"type:varchar(255);column:description;not null;default:'';comment:知识森林描述" json:"description"`
	GraphID        uint       `gorm:"column:graph_id;type:bigint;not null;default:0;comment:'图谱ID';index" json:"graph_id"`
	GraphVersionID uint       `gorm:"column:graph_version_id;type:bigint;not null;default:0;comment:'图谱版本ID';index" json:"graph_version_id"`
	TagName        string     `gorm:"column:tag_name;type:varchar(256);not null;default:'';comment:'标签名称';index" json:"tag_name"`
	TagType        TagType    `gorm:"column:tag_type;type:varchar(16);not null;default:0;comment:'标签类型 tag;edge';index" json:"tag_type"`
	Properties     Properties `gorm:"column:properties;type:text;comment:'标签属性'" json:"properties"`
	TagStatus      TagStatus  `gorm:"column:tag_status;type:varchar(16);not null;default:'not_synced';comment:'标签状态'" json:"tag_status"`
}

func (GraphTag) TableName() string {
	return TableNameKeGraphTag
}

type GraphEdgeTag struct {
	gorm.Model
	GraphID        uint `gorm:"column:graph_id;type:bigint;not null;default:0;comment:'图谱ID';index" json:"graph_id"`
	GraphVersionID uint `gorm:"column:graph_version_id;type:bigint;not null;default:0;comment:'图谱版本ID';index" json:"graph_version_id"`
	EdgeTypeID     uint `gorm:"column:edge_type_id;type:bigint;not null;default:0;comment:'边类型ID';index" json:"edge_type_id"`
	SrcTagID       uint `gorm:"column:src_tag_id;type:bigint;not null;default:0;comment:'起始节点标签ID';index" json:"src_tag_id"`
	DstTagID       uint `gorm:"column:dst_tag_id;type:bigint;not null;default:0;comment:'结束节点标签ID';index" json:"dst_tag_id"`
}

func (GraphEdgeTag) TableName() string {
	return TableNameKeGraphEdgeTag
}

type Property struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Defaults interface{} `json:"defaults"`
	Comment  string      `json:"comment"`
}

func (p Property) TagStr() string {
	str := fmt.Sprintf(" `%s` %s", p.Name, p.Type)
	if p.Defaults != nil {
		str += fmt.Sprintf(" DEFAULT %v", GetValueStr(p.Defaults, p.Type))
	}
	str += fmt.Sprintf(" COMMENT \"%s\"", p.Comment)
	return str
}

type Properties []*Property

func (ps Properties) NameMap() map[string]*Property {
	m := map[string]*Property{}
	for _, p := range ps {
		m[p.Name] = p
	}
	return m
}

func (ps *Properties) Deduplicate() {
	seen := make(map[string]bool)
	result := make(Properties, 0, len(*ps))

	for _, p := range *ps {
		if !seen[p.Name] {
			seen[p.Name] = true
			result = append(result, p)
		}
	}

	*ps = result
}

// ValidateProperties 校验每个Property的Defaults是否符合Type
func (ps Properties) ValidateProperties() error {
	for _, prop := range ps {
		if prop.Defaults == nil {
			continue // 允许nil，后续用类型默认值兜底
		}
		if err := validateValueType(prop.Defaults, prop.Type); err != nil {
			return fmt.Errorf("property '%s' default value type error: %v", prop.Name, err)
		}
	}
	return nil
}

func (ep Properties) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *Properties) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for Properties")
	}
	return json.Unmarshal(bytes, ep)
}

func (t *GraphTag) CreateTagString() string {
	properties := []string{}
	for _, v := range t.Properties {
		properties = append(properties, v.TagStr())
	}

	return fmt.Sprintf("CREATE %s `%s`(%s)", t.TagType, t.TagName, strings.Join(properties, ", "))
}
