package foresttype

import (
	"fmt"

	"github.com/insmtx/corekg/pkgs/plugins/dbplugins"
	"github.com/ygpkg/yg-go/settings"
	"gorm.io/gorm"
)

type DBInstanceType string

const (
	DBInstanceTypeMySQL DBInstanceType = "mysql"
)

type DBInstanceOwnershipType string

const (
	DBInstanceOwnershipTypeSystem   DBInstanceOwnershipType = "system"
	DBInstanceOwnershipTypeExternal DBInstanceOwnershipType = "external"
)

type DBInstanceConnectMode string

const (
	DBInstanceConnectModeStandard DBInstanceConnectMode = "standard"
	DBInstanceConnectModeSSH      DBInstanceConnectMode = "ssh"
)

type DBInstanceConnectionStatus string

const (
	DBInstanceConnectionStatusValid   DBInstanceConnectionStatus = "valid"
	DBInstanceConnectionStatusInvalid DBInstanceConnectionStatus = "invalid"
)

const (
	MysqlDefaultCharset = "utf8mb4"
	MysqlDefaultCollate = "utf8mb4_general_ci"

	MysqlDefaultInstanceAlias = "ke_excel_forest"
)

// ForestDBInstance 数据库实例表结构体
type ForestDBInstance struct {
	gorm.Model
	CompanyID        uint                       `gorm:"column:company_id;type:bigint;not null;default:0;comment:'公司ID';index" json:"company_id"`
	ConnectMode      DBInstanceConnectMode      `gorm:"column:connect_mode;type:varchar(32);not null;;comment:连接模式，standard：标准连接，ssh：ssh隧道模式"`
	ConnectName      string                     `gorm:"column:connect_name;type:varchar(128);not null;;comment:数据库连接名称"`
	ForestID         uint                       `gorm:"column:forest_id;type:bigint;not null;default 0;comment:知识库ID"`
	Host             string                     `gorm:"column:host;type:varchar(255);not null;;comment:数据库地址"`
	InstanceType     dbplugins.DatabaseType     `gorm:"column:instance_type;type:varchar(32);not null;;comment:数据库类型，oracle：oracle，mysql：mysql"`
	OwnershipType    DBInstanceOwnershipType    `gorm:"column:ownership_type;type:varchar(32);not null;;comment:实例归属类型：system-系统自有，external-外部实例"`
	Username         string                     `gorm:"column:username;type:varchar(32);not null;;comment:连接用户名"`
	Password         string                     `gorm:"column:password;type:varchar(128);not null;;comment:连接密码（加密存储）"`
	Port             uint                       `gorm:"column:port;type:int;not null;default 0;comment:端口号"`
	Database         string                     `gorm:"column:database;type:varchar(128);not null;default '';comment:数据库名称"`
	ConnectionStatus DBInstanceConnectionStatus `gorm:"column:connection_status;type:varchar(32);not null;default 'valid';comment:连接状态,valid-有效，invalid-无效"`
	Uin              uint                       `gorm:"column:uin;type:bigint;not null;default 0;comment:用户uin"`
}

type ForestDbInstanceList []ForestDBInstance

func (ForestDBInstance) TableName() string {
	return TableNameKeForestDBInstance
}

func (inst ForestDBInstance) BuildMysqlDNS(dbName, charset string) string {
	return fmt.Sprintf("mysql://%s:%s@%s:%d/%s?charset=%s&parseTime=true&loc=Local", inst.Username, settings.DecryptSecret(inst.Password), inst.Host, inst.Port, dbName, charset)
}

func (l ForestDbInstanceList) ToMap() map[uint]ForestDBInstance {
	m := make(map[uint]ForestDBInstance)
	for _, v := range l {
		m[v.ID] = v
	}
	return m
}
