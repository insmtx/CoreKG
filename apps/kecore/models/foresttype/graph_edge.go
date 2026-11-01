package foresttype

import (
	"fmt"

	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

// GraphEdge 边表
type GraphEdge struct {
	gorm.Model
	GraphID          uint              `gorm:"column:graph_id;type:bigint;not null;default:0;comment:'图谱ID';index" json:"graph_id"`
	GraphVersionID   uint              `gorm:"column:graph_version_id;type:bigint;not null;default:0;comment:'图谱版本ID';index" json:"graph_version_id"`
	SrcID            uint              `gorm:"column:src_id;type:bigint;not null;default:0;comment:'起始节点ID';index" json:"src_id"`
	DstID            uint              `gorm:"column:dst_id;type:bigint;not null;default:0;comment:'结束节点ID';index" json:"dst_id"`
	SrcTagID         uint              `gorm:"column:src_tag_id;type:bigint;not null;default:0;comment:'起始节点标签ID';index" json:"src_tag_id"`
	DstTagID         uint              `gorm:"column:dst_tag_id;type:bigint;not null;default:0;comment:'结束节点标签ID';index" json:"dst_tag_id"`
	TagID            uint              `gorm:"column:tag_id;type:bigint;not null;default:0;comment:'标签ID';index" json:"tag_id"`
	FileIDList       types.UintArray   `gorm:"column:file_id_list;type:text;comment:'文件ID列表'" json:"file_id_list"`
	ChunkIDList      types.StringArray `gorm:"column:chunk_id_list;type:text;comment:'分块ID列表'" json:"chunk_id_list"`
	PropertiesValues PropertiesValues  `gorm:"column:properties_values;type:text;comment:'标签属性'" json:"properties_values"`
}

func (GraphEdge) TableName() string {
	return TableNameKeGraphEdge
}

// GraphEdge 边表
type EdgeInfo struct {
	GraphID          uint              `json:"graph_id"`
	GraphVersionID   uint              `json:"graph_version_id"`
	EdgeID           uint              `json:"edge_id"`
	SrcID            uint              `json:"src_id"`
	DstID            uint              `json:"dst_id"`
	TagID            uint              `json:"tag_id"`
	SrcTagID         uint              `json:"src_tag_id"`
	DstTagID         uint              `json:"dst_tag_id"`
	FileIDList       types.UintArray   `json:"file_id_list"`
	ChunkIDList      types.StringArray `json:"chunk_id_list"`
	EdgeTypeName     string            `json:"edge_type_name"`
	SrcNodeName      string            `json:"src_node_name"`
	DstNodeName      string            `json:"dst_node_name"`
	PropertiesValues PropertiesValues  `json:"properties_values"`
	Properties       Properties        `json:"properties"`
}

func (edge *EdgeInfo) InsertStr() string {
	return fmt.Sprintf("INSERT EDGE `%s`() VALUES \"%s\"->\"%s\":()", edge.EdgeTypeName, edge.SrcNodeName, edge.DstNodeName)
}

// BatchInsertStr 生成批量插入边的 nGQL 语句
// 注意：所有边必须是同一类型（EdgeTypeName 相同）
func BatchInsertStr(edges []*EdgeInfo) string {
	if len(edges) == 0 {
		return ""
	}
	if len(edges) == 1 {
		return edges[0].InsertStr()
	}

	// 构建 VALUES 子句
	values := make([]string, 0, len(edges))
	for _, edge := range edges {
		values = append(values, fmt.Sprintf("\"%s\"->\"%s\":()", edge.SrcNodeName, edge.DstNodeName))
	}

	// 使用第一个边的类型作为边类型（假设所有边都是同一类型）
	edgeType := edges[0].EdgeTypeName
	valuesStr := ""
	for i, v := range values {
		if i > 0 {
			valuesStr += ", "
		}
		valuesStr += v
	}
	return fmt.Sprintf("INSERT EDGE `%s`() VALUES %s", edgeType, valuesStr)
}
