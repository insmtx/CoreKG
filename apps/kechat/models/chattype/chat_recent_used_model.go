/*
 * @Author: morehao morehao@qq.com
 * @Date: 2025-11-21 14:35:28
 * @LastEditors: morehao morehao@qq.com
 * @LastEditTime: 2025-11-21 15:06:10
 * @FilePath: /roc/apps/kechat/models/chattype/chat_recent_used_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package chattype

import (
	"time"

	"gorm.io/gorm"
)

// ChatRecentUsedModel 最近使用的模型表结构体
type ChatRecentUsedModel struct {
	gorm.Model
	Uin        uint       `gorm:"column:uin;type:bigint;not null;;comment:用户ID"`
	CompanyID  uint       `gorm:"column:company_id;type:bigint;not null;default 0;comment:公司ID"`
	ModelID    uint       `gorm:"column:model_id;type:bigint;not null;;comment:模型ID，对应 chat_model.id"`
	LastUsedAt *time.Time `gorm:"column:last_used_at;type:timestamptz;not null;;comment:最近使用时间"`
	UsageCount uint       `gorm:"column:usage_count;type:int;not null;default 1;comment:使用次数"`
}

type ChatRecentUsedModelList []ChatRecentUsedModel

func (ChatRecentUsedModel) TableName() string {
	return TableNameChatRecentUsedModel
}

func (l ChatRecentUsedModelList) ToMap() map[uint]ChatRecentUsedModel {
	m := make(map[uint]ChatRecentUsedModel)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
