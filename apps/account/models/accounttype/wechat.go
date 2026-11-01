package accounttype

import "gorm.io/gorm"

type WechatBinding struct {
	gorm.Model

	// AppID 微信小程序的AppID
	AppID string `gorm:"column:app_id;type:varchar(100);not null"`
	// OpenID 微信小程序的OpenID
	OpenID string `gorm:"column:open_id;type:varchar(100);not null;unique"`
	// UnionID 微信小程序的UnionID
	UnionID string `gorm:"column:union_id;type:varchar(100);not null"`

	Subscribe     int32  `gorm:"column:subscribe;type:smallint;default:0"`
	Nickname      string `gorm:"column:nickname;type:varchar(255);default:''"`
	Sex           int32  `gorm:"column:sex;type:smallint;default:0"`
	City          string `gorm:"column:city;type:varchar(255);default:''"`
	Country       string `gorm:"column:country;type:varchar(255);default:''"`
	Province      string `gorm:"column:province;type:varchar(255);default:''"`
	Headimgurl    string `gorm:"column:headimgurl;type:varchar(255);default:''"`
	SubscribeTime int32  `gorm:"column:subscribe_time;type:int;default:0"`
}

func (WechatBinding) TableName() string {
	return TableNameWechatBinding
}
