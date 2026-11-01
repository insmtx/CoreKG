package foresttype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

// GraphNode 节点表
type GraphNode struct {
	gorm.Model
	Uin            uint   `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID      uint   `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	GraphID        uint   `gorm:"column:graph_id;type:bigint;not null;default:0;comment:'图谱ID';index" json:"graph_id"`
	GraphVersionID uint   `gorm:"column:graph_version_id;type:bigint;not null;default:0;comment:'图谱版本ID';index" json:"graph_version_id"`
	Name           string `gorm:"column:name;type:varchar(512);not null;default:'';comment:'节点ID';uniqueIndex:idx_tag_node,priority:2" json:"name"`
}

func (GraphNode) TableName() string {
	return TableNameKeGraphNode
}

// GraphTagNode 节点表
type GraphTagNode struct {
	gorm.Model
	GraphID        uint `gorm:"column:graph_id;type:bigint;not null;default:0;comment:'图谱ID';index" json:"graph_id"`
	GraphVersionID uint `gorm:"column:graph_version_id;type:bigint;not null;default:0;comment:'图谱版本ID';index" json:"graph_version_id"`
	TagID          uint `gorm:"column:tag_id;type:bigint;not null;default:0;comment:'标签ID';uniqueIndex:idx_tag_node,priority:1" json:"tag_id"`
	// NodeID           uint              `gorm:"column:node_id;type:bigint;not null;default:0;comment:'节点ID';uniqueIndex:idx_tag_node,priority:2" json:"node_id"`
	FileIDList       types.UintArray   `gorm:"column:file_id_list;type:text;comment:'文件ID列表'" json:"file_id_list"`
	ChunkIDList      types.StringArray `gorm:"column:chunk_id_list;type:text;comment:'分块ID列表'" json:"chunk_id_list"`
	PropertiesValues PropertiesValues  `gorm:"column:properties_values;type:text;comment:'标签属性'" json:"properties_values"`

	Uin         uint        `gorm:"column:uin;type:bigint;not null;default:0;comment:'用户ID';index" json:"uin"`
	CompanyID   uint        `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	Name        string      `gorm:"column:name;type:varchar(512);not null;default:'';comment:'节点ID';uniqueIndex:idx_tag_node,priority:2" json:"name"`
	CreatedType CreatedType `gorm:"column:created_type;type:varchar(63);not null;default:'algorithm';comment:'创建类型 algorithm:算法创建,manual:手动创建'" json:"created_type"`
}

func (GraphTagNode) TableName() string {
	return TableNameKeGraphTagNode
}

type CreatedType string

const (
	CreatedTypeAlgorithm CreatedType = "algorithm"
	CreatedTypeManual    CreatedType = "manual"
)

type PropertyValue struct {
	Name     string            `json:"name"`
	Value    interface{}       `json:"value"`
	ChunkIDs types.StringArray `json:"chunk_ids"`
}

type PropertiesValues []*PropertyValue

func (ep PropertiesValues) NameMap() map[string]*PropertyValue {
	result := map[string]*PropertyValue{}
	for _, v := range ep {
		result[v.Name] = v
	}
	return result
}

func (ep PropertiesValues) Value() (driver.Value, error) {
	return json.Marshal(ep)
}

func (ep *PropertiesValues) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid type for PropertiesValues")
	}
	return json.Unmarshal(bytes, ep)
}

// Validate 根据Properties定义校验PropertiesValues
func (pvs PropertiesValues) Validate(properties Properties) error {
	// 创建Property映射，方便查找
	propMap := properties.NameMap()

	// 校验每个PropertyValue
	for _, pv := range pvs {
		prop, exists := propMap[pv.Name]
		if !exists {
			return fmt.Errorf("property '%s' is not defined", pv.Name)
		}

		if err := validateValueType(pv.Value, prop.Type); err != nil {
			return fmt.Errorf("property '%s': %v", pv.Name, err)
		}
	}

	return nil
}

// validateValueType 校验值类型是否匹配
func validateValueType(value interface{}, expectedType string) error {
	if value == nil {
		return nil // 允许nil值
	}

	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "int64":
		// 支持int类型和可以转换为int的字符串
		switch v := value.(type) {
		case int, int8, int16, int32, int64:
			return nil
		case float64:
			// 检查是否为整数
			if v != float64(int64(v)) {
				return fmt.Errorf("float64 value %f is not an integer", v)
			}
			return nil
		case string:
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				return fmt.Errorf("cannot convert string '%s' to integer", v)
			}
		default:
			return fmt.Errorf("expected int64, got %T", value)
		}
	case "double":
		switch v := value.(type) {
		case float32, float64:
			return nil
		case int, int8, int16, int32, int64:
			return nil
		case uint, uint8, uint16, uint32, uint64:
			return nil
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return fmt.Errorf("cannot convert string '%s' to float", v)
			}
		default:
			return fmt.Errorf("expected float, got %T", value)
		}
	case "bool":
		switch v := value.(type) {
		case bool:
			return nil
		case string:
			if v == "true" || v == "false" || v == "1" || v == "0" {
				return nil
			}
			return fmt.Errorf("cannot convert string '%s' to boolean", v)
		case int:
			if v == 0 || v == 1 {
				return nil
			}
			return fmt.Errorf("cannot convert integer '%d' to boolean", v)
		default:
			return fmt.Errorf("expected boolean, got %T", value)
		}
	default:
		// 对于自定义类型，只做基本的类型检查
		return fmt.Errorf("unsupported type: %s", expectedType)
	}

	return nil
}

// GetValueStr 获取值的字符串类型
func GetValueStr(value interface{}, expectedType string) string {
	if value == nil {
		return "NULL" // 允许nil值
	}
	switch expectedType {
	case "string":
		vstr, ok := value.(string)
		if !ok {
			return ""
		}
		return fmt.Sprintf("\"%s\"", vstr)
	case "int64":
		// 支持int类型和可以转换为int的字符串
		switch v := value.(type) {
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%d", v)
		case string:
			vint, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return "0"
			}
			return fmt.Sprintf("%d", vint)
		default:
			return "0"
		}
	case "double":
		switch v := value.(type) {
		case float32, float64:
			return fmt.Sprintf("%f", v)
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%f", v)
		case uint, uint8, uint16, uint32, uint64:
			return fmt.Sprintf("%f", v)
		case string:
			vdouble, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return "0.0"
			}
			return fmt.Sprintf("%f", vdouble)
		default:
			return "0.0"
		}
	case "bool":
		switch v := value.(type) {
		case bool:
			return fmt.Sprintf("%t", v)
		case string:
			// 支持字符串转bool
			switch strings.ToLower(v) {
			case "true", "1", "yes", "on":
				return "true"
			case "false", "0", "no", "off":
				return "false"
			default:
				return "false" // 默认返回false
			}
		case int:
			if v == 1 {
				return "true"
			} else if v == 0 {
				return "false"
			} else {
				return "false" // 其他数字默认返回false
			}
		default:
			return "false"
		}
	default:
		// 对于自定义类型，只做基本的类型检查
		return "NULL"
	}
}

// ValidateAndComplete 根据Properties定义校验PropertiesValues，并补全缺失的字段
func (pvs PropertiesValues) ValidateAndComplete(tag *GraphTag) (PropertiesValues, error) {
	// 创建Property映射，方便查找
	propMap := tag.Properties.NameMap()

	// 创建现有PropertyValue的映射
	valueMap := pvs.NameMap()

	// 校验现有的PropertyValue
	for _, pv := range pvs {
		prop, exists := propMap[pv.Name]
		if !exists {
			// 如果属性不存在，添加一个属性
			return nil, fmt.Errorf("property '%s' not found", pv.Name)
		}

		if err := validateValueType(pv.Value, prop.Type); err != nil {
			return nil, fmt.Errorf("property '%s': %v", pv.Name, err)
		}
	}

	// 补全缺失的字段
	result := make(PropertiesValues, 0, len(tag.Properties))

	// 先添加已有的值
	result = append(result, pvs...)

	// 添加缺失的字段并赋予默认值
	for _, prop := range tag.Properties {
		if _, exists := valueMap[prop.Name]; !exists {
			defaultValue := prop.Defaults
			// 如果 defaults 为空，用类型默认值兜底
			if defaultValue == nil {
				defaultValue = getDefaultValueByType(prop.Type)
			}
			result = append(result, &PropertyValue{
				Name:  prop.Name,
				Value: defaultValue,
			})
		}
	}

	return result, nil
}

// getDefaultValueByType 根据类型获取默认值
func getDefaultValueByType(typ string) interface{} {
	switch typ {
	case "string":
		return ""
	case "int64":
		return int64(0)
	case "double":
		return float64(0.0)
	case "bool":
		return false
	default:
		// 对于未知类型，返回nil
		return nil
	}
}

// GetType 获取值的实际类型字符串
func GetType(value interface{}) string {
	if value == nil {
		return "string"
	}
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64:
		return "int"
	case uint, uint8, uint16, uint32, uint64:
		return "uint"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case []interface{}:
		return "array"
	default:
		return "string"
	}
}

// 更新PropertiesValues并与Properties对比
func (currentValues *PropertiesValues) UpdateAndSyncProperties(properties Properties, newValues PropertiesValues) {
	// 创建一个map来快速查找properties中的属性
	propertyMap := properties.NameMap()

	// 创建一个map来存储当前的值
	valueMap := currentValues.NameMap()

	// 更新现有的值
	for _, newVal := range newValues {
		if newVal != nil {
			if existingVal, exists := valueMap[newVal.Name]; exists {
				p := propertyMap[newVal.Name]
				if p.Type == "string" {
					// 空值检查
					if newVal.Value == nil {
						continue
					}
					newStr, ok := newVal.Value.(string)
					if !ok {
						continue
					}
					// 如果现有值为空，直接赋值
					if existingVal.Value == nil {
						existingVal.Value = newStr
						continue
					}
					existingStr, ok := existingVal.Value.(string)
					if !ok {
						existingVal.Value = newStr
						continue
					}
					// 相同值检查
					if newStr == existingStr || (newStr == p.Defaults && existingStr == p.Defaults) {
						continue
					}
					// 避免重复添加
					if !strings.Contains(existingStr, newStr) {
						// 👇 修改点在这里：只在非空时才加 "&&&"
						if existingStr == "" {
							existingVal.Value = newStr
						} else {
							existingVal.Value = existingStr + "&&&" + newStr
						}
					}
					continue
				}
				// 更新现有值
				existingVal.Value = newVal.Value
				for _, v := range newVal.ChunkIDs.Slice() {
					existingVal.ChunkIDs.Add(v)
				}
				existingVal.ChunkIDs.RemoveDuplicates()
			} else {
				// 添加新值
				valueMap[newVal.Name] = newVal
			}
		}
	}

	// 转换回slice
	result := make(PropertiesValues, 0, len(valueMap))
	for _, val := range valueMap {
		result = append(result, val)
	}
	*currentValues = result
}

// TagNodeInfo 跟tag详情相关的节点信息
type TagNodeInfo struct {
	Name             string            `json:"name"`
	NodeID           uint              `json:"node_id"`
	NodeTagID        uint              `json:"node_tag_id"`
	Uin              uint              `json:"uin"`
	CompanyID        uint              `json:"company_id"`
	GraphID          uint              `json:"graph_id"`
	GraphVersionID   uint              `json:"graph_version_id"`
	TagID            uint              `json:"tag_id"`
	FileIDList       types.UintArray   `json:"file_id_list"`
	ChunkIDList      types.StringArray `json:"chunk_id_list"`
	PropertiesValues PropertiesValues  `json:"properties_values"`
	Properties       Properties        `json:"properties"`
	TagName          string            `json:"tag_name"`
}

func (tni *TagNodeInfo) InsertStr() string {
	nameMap := tni.Properties.NameMap()
	pstr := []string{}
	vstr := []string{}
	for _, v := range tni.PropertiesValues {
		p, ok := nameMap[v.Name]
		if ok {
			pstr = append(pstr, fmt.Sprintf("`%s`", v.Name))
			vstr = append(vstr, GetValueStr(v.Value, p.Type))
		}
	}

	return fmt.Sprintf("INSERT VERTEX `%s`(%s) VALUES \"%s\":(%s)", tni.TagName, strings.Join(pstr, ","), tni.Name, strings.Join(vstr, ","))
}
