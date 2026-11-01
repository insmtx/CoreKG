package saletype

import (
	"gorm.io/gorm"
)

// SaleOrderItem 订单明细表结构体
type SaleOrderItem struct {
	gorm.Model
	OrderID        uint   `gorm:"column:order_id;type:bigint;not null;;comment:订单ID"`
	ProductID      uint   `gorm:"column:product_id;type:bigint;not null;;comment:商品ID"`
	ProductName    string `gorm:"column:product_name;type:varchar(64);not null;;comment:商品名称（冗余字段，便于查询）"`
	BusinessSource string `gorm:"column:business_source;type:varchar(32);;;comment:业务来源，knowledge：知识引擎"`
	Price          int64  `gorm:"column:price;type:bigint;not null;default 0;comment:商品价格（分）"`
	Quantity       uint   `gorm:"column:quantity;type:bigint;not null;default 1;comment:购买数量"`
	TotalAmount    int64  `gorm:"column:total_amount;type:bigint;not null;default 0;comment:订单总金额（分）"`
	PaymentAmount  int64  `gorm:"column:payment_amount;type:bigint;not null;default 0;comment:支付金额（实付金额，分）"`
}

type SaleOrderItemList []SaleOrderItem

func (SaleOrderItem) TableName() string {
	return TableNameSaleOrderItem
}

func (l SaleOrderItemList) ToMap() map[uint]SaleOrderItem {
	m := make(map[uint]SaleOrderItem)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
