package wecoms

import (
	"context"
	"errors"

	"github.com/insmtx/corekg/apps/account/models/accounttype"
	"github.com/xen0n/go-workwx"
	"github.com/ygpkg/yg-go/logs"
	"github.com/ygpkg/yg-go/types"
	"gorm.io/gorm"
)

type Department struct {
	CompanyID string `json:"company_id" gorm:"column:company_id;type:varchar(32)"`
	// DeptID 部门 ID
	DeptID int64 `json:"dept_id" gorm:"column:dept_id"`
	// Name 部门名称
	Name string `json:"name" gorm:"column:name;type:varchar(32)"`
	// ParentID 父亲部门id。根部门为1
	ParentID int64 `json:"parent_id" gorm:"column:parent_id"`
	// Order 在父部门中的次序值。order值大的排序靠前。值范围是[0, 2^32)
	Order uint32 `json:"order" gorm:"column:order"`
}

func (*Department) TableName() string { return TableNameDept }
func (*Department) Indexes() string {
	return TableNameDept
}

type User struct {
	ID string `json:"id" gorm:"column:id;type:varchar(32);primarykey"`

	CompanyID string `json:"company_id" gorm:"column:company_id;type:varchar(32)"`
	// Name 成员名称
	Name string `json:"name" gorm:"column:name;type:varchar(12)"`
	// Position 职务信息；第三方仅通讯录应用可获取
	Position string `json:"position" gorm:"column:position;type:varchar(32)"`
	// Mobile 手机号码；第三方仅通讯录应用可获取
	Mobile string `json:"mobile" gorm:"column:mobile;type:varchar(20)"`
	// Gender 性别
	Gender accounttype.UserGender `json:"gender" gorm:"column:gender"`
	// Email 邮箱；第三方仅通讯录应用可获取
	Email string `json:"email" gorm:"column:email;type:varchar(32)"`
	// AvatarURL 头像 URL；第三方仅通讯录应用可获取
	// NOTE：如果要获取小图将url最后的”/0”改成”/100”即可。
	AvatarURL string `json:"avatar_url" gorm:"column:avatar_url;type:varchar(64)"`
	// Telephone 座机；第三方仅通讯录应用可获取
	Telephone string `json:"telephone" gorm:"column:telephone;type:varchar(20)"`
	// IsEnabled 成员的启用状态
	IsEnabled types.Bool `json:"is_enable" gorm:"column:is_enable"`
	// Alias 别名；第三方仅通讯录应用可获取
	Alias string `json:"alias" gorm:"column:alias"`
	// Status 成员激活状态
	Status workwx.UserStatus `json:"status" gorm:"column:status"`
	// QRCodeURL 员工个人二维码；第三方仅通讯录应用可获取
	// 扫描可添加为外部联系人
	QRCodeURL string `json:"qr_code_url" gorm:"column:qr_code_url;type:varchar(64)"`
}

func (*User) TableName() string { return TableNameUser }

type RelDeptUser struct {
	CompanyID string `json:"company_id" gorm:"column:company_id;type:varchar(32);index:idx_rel_dept_user,unique"`
	// DeptID 部门 ID
	DeptID int64  `json:"dept_id" gorm:"column:dept_id;index:idx_rel_dept_user,unique"`
	UserID string `json:"user_id" gorm:"column:user_id;type:varchar(32);index:idx_rel_dept_user,unique"`

	Order uint32 `json:"order" gorm:"column:order"`
	// IsLeader 在所在的部门内是否为上级
	IsLeader types.Bool `json:"is_leader" gorm:"column:is_leader;type:smallint"`
}

func (*RelDeptUser) TableName() string { return TableNameRelDeptUser }

func ListAllDepartment(wxcli *workwx.WorkwxApp) ([]*Department, error) {
	depts, err := wxcli.ListAllDepts()
	if err != nil {
		return nil, err
	}
	rets := make([]*Department, 0, len(depts))
	for _, dept := range depts {
		rets = append(rets, &Department{
			CompanyID: wxcli.CorpID,
			DeptID:    dept.ID,
			Name:      dept.Name,
			ParentID:  dept.ParentID,
			Order:     dept.Order,
		})
	}

	return rets, nil
}

func SyncAllDepartment(wxcli *workwx.WorkwxApp, db *gorm.DB) error {
	depts, err := ListAllDepartment(wxcli)
	if err != nil {
		return err
	}
	ctx := context.TODO()
	for _, dept := range depts {
		sql := db.Table(TableNameDept).
			Where("company_id = ? AND dept_id = ?", dept.CompanyID, dept.DeptID)
		tDept := &Department{}
		if err := sql.Find(tDept).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(dept).Error; err != nil {
					logs.ErrorContextf(ctx, "[wecoms] create dept(%s_%v) failed, %s", dept.CompanyID, dept.DeptID, err)
				}
			} else {
				logs.ErrorContextf(ctx, "[wecoms] get dept(%s_%v) failed, %s", dept.CompanyID, dept.DeptID, err)
			}
		} else {
			if err := sql.Updates(dept).Error; err != nil {
				logs.ErrorContextf(ctx, "[wecoms] update dept(%s_%v) failed, %s", dept.CompanyID, dept.DeptID, err)
			}
		}
		if err := syncUser(wxcli, db, dept.DeptID); err != nil {
			logs.ErrorContextf(ctx, "[wecoms] syncUser failed for dept: %v, %s", dept.DeptID, err)
		}
	}
	return nil
}

func syncUser(wxcli *workwx.WorkwxApp, db *gorm.DB, deptID int64) error {
	ctx := context.TODO()
	wxusers, err := wxcli.ListUsersByDeptID(deptID, false)
	if err != nil {
		return err
	}
	for _, wxuser := range wxusers {
		u := &User{
			ID:        (wxuser.UserID),
			CompanyID: wxcli.CorpID,
			Name:      wxuser.Name,
			Position:  wxuser.Position,
			Mobile:    wxuser.Mobile,
			Gender:    accounttype.UserGender(wxuser.Gender),
			Email:     wxuser.Email,
			AvatarURL: wxuser.AvatarURL,
			Telephone: wxuser.Telephone,
			IsEnabled: types.NewBool(wxuser.IsEnabled),
			Alias:     wxuser.Alias,
			Status:    wxuser.Status,
			QRCodeURL: wxuser.QRCodeURL,
		}
		depts := make([]*RelDeptUser, 0, len(wxuser.Departments))
		for _, dept := range wxuser.Departments {
			depts = append(depts, &RelDeptUser{
				CompanyID: wxcli.CorpID,
				DeptID:    dept.DeptID,
				UserID:    wxuser.UserID,
				// IsLeader 在所在的部门内是否为上级
				IsLeader: types.NewBool(dept.IsLeader),
				Order:    dept.Order,
			})
		}

		if err := createOrUpdateUser(db.WithContext(ctx), u, depts); err != nil {
			logs.ErrorContextf(ctx, "[wecoms] createOrUpdateUser failed, %s", err)
		}
	}
	return nil
}

func createOrUpdateUser(db *gorm.DB, user *User, depts []*RelDeptUser) error {
	ctx := context.TODO()
	sql := db.Table(TableNameUser).WithContext(ctx).
		Where("company_id = ? AND id = ?", user.CompanyID, user.ID)
	tUser := &User{}
	if err := sql.Find(tUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(user).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if err := sql.Updates(user).Error; err != nil {
			logs.ErrorContextf(ctx, "[wecoms] update user failed ,%s", err)
			return err
		}
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Table(TableNameRelDeptUser).
			Where("company_id = ? AND user_id = ?", user.CompanyID, user.ID).
			Scopes().Delete(nil).Error
		if err != nil {
			return err
		}
		err = tx.Create(&depts).Error
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logs.ErrorContextf(ctx, "[wecoms] update %s failed, %s", TableNameRelDeptUser, err)
		return err
	}

	return nil
}
