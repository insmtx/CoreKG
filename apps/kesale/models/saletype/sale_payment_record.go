package saletype

import (
	"time"

	"gorm.io/gorm"
)

// SalePaymentRecord 支付记录表结构体
type SalePaymentRecord struct {
	gorm.Model
	OrderID        uint       `gorm:"column:order_id;type:bigint;not null;;comment:订单ID"`
	OrderSn        string     `gorm:"column:order_sn;type:varchar(64);;;comment:订单号（冗余字段，便于查询）"`
	CompanyID      uint       `gorm:"column:company_id;type:bigint;;;comment:公司ID（冗余字段，便于查询）"`
	Uin            uint       `gorm:"column:uin;type:bigint;;;comment:用户ID（冗余字段，便于查询）"`
	RecordSn       string     `gorm:"column:record_sn;type:varchar(64);not null;;comment:支付记录号"`
	RecordType     string     `gorm:"column:record_type;type:varchar(16);not null;;comment:记录类型：payment-支付，refund-退款"`
	BusinessSource string     `gorm:"column:business_source;type:varchar(32);;;comment:业务来源，knowledge：知识引擎"`
	Amount         int64      `gorm:"column:amount;type:bigint;not null;default 0;comment:金额（分）"`
	PaymentChannel string     `gorm:"column:payment_channel;type:varchar(32);;;comment:支付渠道：alipay-支付宝，wechat-微信，bank-银行转账等"`
	TradeID        string     `gorm:"column:trade_id;type:varchar(128);;;comment:三方交易ID"`
	TradeNo        string     `gorm:"column:trade_no;type:varchar(128);;;comment:三方交易号"`
	Status         string     `gorm:"column:status;type:varchar(16);not null;default pending;comment:流水状态：pending-处理中，success-成功，failed-失败"`
	PaidAt         *time.Time `gorm:"column:paid_at;type:timestamptz;;;comment:支付时间"`
	RequestParams  string     `gorm:"column:request_params;type:text;;;comment:调用三方入参（JSON格式）"`
	ResponseData   string     `gorm:"column:response_data;type:text;;;comment:调用三方出参（JSON格式）"`
	CallbackData   string     `gorm:"column:callback_data;type:text;;;comment:支付回调信息（JSON格式）：以最后一次回调内容为准"`
	Remark         string     `gorm:"column:remark;type:varchar(512);;;comment:备注"`
}

type SalePaymentRecordList []SalePaymentRecord

func (SalePaymentRecord) TableName() string {
	return TableNameSalePaymentRecord
}

func (l SalePaymentRecordList) ToMap() map[uint]SalePaymentRecord {
	m := make(map[uint]SalePaymentRecord)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
