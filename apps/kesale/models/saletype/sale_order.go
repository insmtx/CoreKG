package saletype

import (
	"time"

	"gorm.io/gorm"
)

// SaleOrder 订单表结构体
type SaleOrder struct {
	gorm.Model
	CompanyID      uint       `gorm:"column:company_id;type:bigint;not null;;comment:公司ID"`
	Uin            uint       `gorm:"column:uin;type:bigint;not null;;comment:购买人员ID（user_identification的ID）"`
	OrderSn        string     `gorm:"column:order_sn;type:varchar(64);not null;;comment:订单号"`
	TotalAmount    int64      `gorm:"column:total_amount;type:bigint;not null;default 0;comment:订单总金额（分）"`
	PaymentAmount  int64      `gorm:"column:payment_amount;type:bigint;not null;default 0;comment:支付金额（实付金额，分）"`
	PaymentChannel string     `gorm:"column:payment_channel;type:varchar(32);;;comment:支付渠道：wechat-微信，alipay-支付宝，bank-银行转账等"`
	OrderStatus    string     `gorm:"column:order_status;type:varchar(16);not null;default pending;comment:订单状态：pending-待支付，success-已完成，closed-已关闭，cancelled-已取消"`
	BusinessSource string     `gorm:"column:business_source;type:varchar(32);;;comment:业务来源，knowledge：知识引擎"`
	BusinessType   string     `gorm:"column:business_type;type:varchar(32);;;comment:业务类型，例如 subscription：订阅付费、资源付费等类型，维护后期订单回调处理"`
	PaidAt         *time.Time `gorm:"column:paid_at;type:timestamptz;;;comment:成功支付时间"`
}

type SaleOrderList []SaleOrder

func (SaleOrder) TableName() string {
	return TableNameSaleOrder
}

func (l SaleOrderList) ToMap() map[uint]SaleOrder {
	m := make(map[uint]SaleOrder)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
