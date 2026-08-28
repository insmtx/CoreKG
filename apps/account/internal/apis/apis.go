package apis

import (
	"github.com/insmtx/corekg/apps/account/accountmds"
	"github.com/insmtx/corekg/apps/account/internal/apis/apikey"
	"github.com/insmtx/corekg/apps/account/internal/apis/privite"
	"github.com/insmtx/corekg/apps/kecore/services/deployhandle"

	"github.com/ygpkg/yg-go/apis/runtime/server"
	"github.com/ygpkg/yg-go/metrics"
)

// RegistryRouter .
func RegistryRouter(eng *server.Router) {
	eng.HandleDoc("account")
	eng.G("account.metrics", metrics.GetHandler())
	eng.AuthInject("", accountmds.InjectLoginStatus)

	eng.G("account.Auth", ForwardAuth)

	// 上传文件
	eng.PRequireLogin("account.UploadCustomerImage", UploadCustomerImage)

	eng.Any("account.HandleWechatMpMessage", HandleWechatMpMessage)

	eng.P("account.LoginThird", LoginThird)
	eng.P("account.LoginByEmail", LoginByEmail)
	eng.P("account.RegisterThird", RegisterThird)
	eng.P("account.ChooseUin", ChooseUin)
	eng.PRequireLogin("account.CLIAuthApprove", CLIAuthApprove)
	eng.PRequireLogin("account.CLIAuthDeny", CLIAuthDeny)
	eng.P("account.GetOBOToken", GetOBOToken)
	eng.P("account.LoginByPassword", accountmds.DecryptMD("request.password"), LoginByPassword)

	eng.P("account.RequestPasswordResetCode", RequestPasswordResetCode)
	eng.P("account.ForgotPassword", accountmds.DecryptMD("request.password"), ForgotPassword)

	eng.PRequireLogin("account.Profile", UserProfile)
	eng.PRequireLogin("account.CheckUserIdentifyExist", CheckUserIdentifyExist)
	// 个人实名认证
	eng.PRequireLogin("account.PersonAuth", PersonAuth)
	eng.PRequireLogin("account.ListPersonAuth", ListPersonAuth)
	eng.PRequireLogin("account.ReviewPersonAuth", ReviewPersonAuth)
	// 认证公司
	eng.PRequireLogin("account.CompanyAuth", CompanyAuth)
	eng.PRequireLogin("account.ListCompany", ListCompany)
	eng.PRequireLogin("account.GetCompany", GetCompany)
	eng.PRequireLogin("account.ReviewCompanyAuth", ReviewCompanyAuth)
	eng.PRequireLogin("account.GetBindCompanyKey", GetBindCompanyKey)
	eng.P("account.BindCompany", BindCompany)
	eng.P("account.CreateCompany", CreateCompany)

	//ke invite with permset
	eng.PRequireLogin("account.GetBindCompanyKeyWithPermSet", accountmds.RequireSysAdminRole, GetBindCompanyKeyWithPermSet)
	eng.P("account.BindCompanyWithPermSet", accountmds.EmployeeQuotaBindMD, accountmds.DecryptMD("request.password"), BindCompanyWithPermSet)
	eng.P("account.GetInviteInfo", GetInviteInfo)

	// 用户中心
	eng.PRequireLogin("account.DetailPersonalCenter", DetailPersonalCenter)
	eng.PRequireLogin("account.ListUin", ListUin)
	eng.PRequireLogin("account.SwitchLogin", SwitchLogin)
	eng.PRequireLogin("account.UpdatePhoneVerifyCode", UpdatePhoneVerifyCode)
	eng.PRequireLogin("account.UpdatePhoneSendCode", UpdatePhoneSendCode)
	eng.PRequireLogin("account.BindUserWechat", BindUserWechat)
	eng.PRequireLogin("account.UpdateAccountPassword", accountmds.DecryptMD("request.old_password", "request.new_password"), UpdateAccountPassword)
	eng.PRequireLogin("account.UpdateUserInfo", UpdateUserInfo)
	// API key
	eng.PRequireLogin("account.ListAPIKey", ListAPIKey)
	eng.PRequireLogin("account.CreateAPIKey", CreateAPIKey)
	eng.PRequireLogin("account.DeleteAPIKey", DeleteAPIKey)

	// 员工管理
	eng.PRequireEmployee("account.ListEmployee", accountmds.RequireOpPrivilege, ListEmployee)
	eng.PRequireEmployee("account.GetEmployeeDetail", accountmds.RequireOpPrivilege, GetEmployeeDetail)
	eng.PRequireEmployee("account.UpdateEmployee", accountmds.RequireOpPrivilege, UpdateEmployee)
	//remove opmd require, delete need to be an admin first
	eng.PRequireLogin("account.DeleteEmployee", accountmds.RequireSysAdminRole, DeleteEmployee)
	eng.PRequireLogin("account.ListEmployeeNickID", ListEmployeeNickID)
	eng.PRequireLogin("account.GetCompanyAdmins", GetCompanyAdmins)

	// 职位权限
	eng.PRequireEmployee("account.ListPosition", accountmds.RequireOpPrivilege, ListPosition)
	eng.PRequireEmployee("account.CreatePosition", accountmds.RequireOpPrivilege, CreatePosition)
	eng.PRequireEmployee("account.GetPositionDetail", accountmds.RequireOpPrivilege, GetPositionDetail)
	eng.PRequireEmployee("account.ModifyPositionInfo", accountmds.RequireOpPrivilege, ModifyPositionInfo)
	eng.PRequireEmployee("account.ModifyPositionPrivilege", accountmds.RequireOpPrivilege, ModifyPositionPrivilege)
	eng.PRequireEmployee("account.DeletePosition", accountmds.RequireOpPrivilege, DeletePosition)

	eng.PRequireEmployee("account.ListPrivilege", accountmds.RequireOpPrivilege, ListPrivilege)
	eng.PRequireEmployee("account.CreatePrivilege", accountmds.RequireOpPrivilege, CreatePrivilege)
	eng.PRequireEmployee("account.ModifyPrivilege", accountmds.RequireOpPrivilege, ModifyPrivilege)
	eng.PRequireEmployee("account.DeletePrivilege", accountmds.RequireOpPrivilege, DeletePrivilege)

	// 获取个人权限
	eng.PRequireEmployee("account.GetMyAction", GetMyAction)

	// 私有化
	eng.P("account.LoginByPasswordPrivate", accountmds.DecryptMD("request.password"), privite.LoginByPasswordPrivate)

	eng.PRequireLogin("account.CreateEmployee", accountmds.RequireSysAdminRole, accountmds.DecryptMD("request.password"), privite.CreateEmployee)
	eng.PRequireLogin("account.EditEmployee", accountmds.RequireSysAdminRole, accountmds.DecryptMD("request.password"), privite.EditEmployee)
	eng.PRequireLogin("account.DeleteEmployeePrivate", accountmds.RequireSysAdminRole, privite.DeleteEmployeePrivate)

	//agent key
	eng.PRequireLogin("account.CreateAgentApiKey", apikey.CreateAgentApiKey)
	eng.PRequireLogin("account.ListAgentAPIKey", apikey.ListAgentAPIKey)
	eng.PRequireLogin("account.DeleteAgentApikey", apikey.DeleteAgentApikey)
	eng.PRequireLogin("account.SetAgentApiKeyStatus", apikey.SetAgentApiKeyStatus)

	//password auth
	eng.PRequireLogin("account.CheckPassword", CheckPassword)

	// 获取登录配置
	eng.P("account.GetLoginSetting", GetLoginSetting)

	// Connect user account with external platform via OAuth2
	eng.PRequireLogin("account.PreConnect", PreConnect)
	eng.G("account.Connect/:provider", Connect)
	eng.G("account.Connect/callback", Callback)
	eng.PRequireLogin("account.Bindings", ListBindings)
	eng.PRequireLogin("account.Unbind", Unbind)

	//organization
	eng.PRequireLogin("account.CreateDepartment", accountmds.RequireSysAdminRole, CreateDepartment)
	eng.PRequireLogin("account.DeleteDepartment", accountmds.RequireSysAdminRole, DeleteDepartment)
	eng.PRequireLogin("account.RenameDepartment", accountmds.RequireSysAdminRole, RenameDepartment)
	eng.PRequireLogin("account.MoveDepartment", accountmds.RequireSysAdminRole, MoveDepartment)
	eng.PRequireLogin("account.GetDepartmentTree", GetDepartmentTree)
	//saas
	eng.PRequireLogin("account.CreateDepartmentEmployee", accountmds.RequireSysAdminRole, accountmds.EmployeeQuotaMD, CreateDepartmentEmployee)
	eng.PRequireLogin("account.EditDepartmentEmployee", accountmds.RequireSysAdminRole, EditDepartmentEmployee)
	//private
	eng.PRequireLogin("account.CreateDepartmentEmployeePrivate", accountmds.RequireSysAdminRole, CreateDepartmentEmployeePrivate)
	eng.PRequireLogin("account.EditDepartmentEmployeePrivate", accountmds.RequireSysAdminRole, EditDepartmentEmployeePrivate)

	//organization setting
	eng.PRequireLogin("account.EditCompanyInfo", accountmds.RequireSysAdminRole, EditCompanyInfo)
	eng.PRequireLogin("account.UploadOrganizeLogo", accountmds.RequireSysAdminRole, UploadOrganizeLogo)
	eng.PRequireLogin("account.ResetPassword", accountmds.RequireSysAdminRole, ResetPassword)
	eng.PRequireLogin("account.GetCompanyInfo", GetCompanyInfo)

	//change password notice
	//use refresh token to valid
	eng.P("account.ChangePasswordNotice", accountmds.RequireRefreshToken, ChangePasswordNotice)
	eng.P("account.ChangeDefaultPassword", accountmds.RequireRefreshToken, accountmds.DecryptMD("request.old_password", "request.new_password"), ChangeDefaultPassword)

	eng.P("account.SwitchPrivateEvn", deployhandle.SwitchPrivateEvn)
	eng.P("account.NowDeployMode", deployhandle.NowDeployMode)

	eng.PRequireLogin("account.UploadWebSiteLogo", accountmds.RequireSysAdminRole, UploadWebSiteLogo)
	//
	eng.P("account.GetGlobalInfo", GetGlobalInfo)

	eng.PRequireLogin("account.UpdateWebsiteInfo", accountmds.RequireSysAdminRole, UpdateWebsiteInfo)
}
