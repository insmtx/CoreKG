import { send, upload } from './request'

/** 创建企业 */
export const createCompany = (data: {
  company_name: string
  user_display_name: string
  domain_name: string
  refresh_token?: string // 登录页面创建时需要，切换组织弹窗创建时不需要（使用请求头token）
  user_id?: number
}) => send('account.CreateCompany', data)

// 获取登录配置
export const getLoginConfig = (body) => send('account.GetLoginSetting', body)
// 账号密码登录
export const loginByPassword = (body) => send('account.LoginByPassword', body)
// 私有化账号密码登录
export const loginByPasswordPrivate = (body) =>
  send('account.LoginByPasswordPrivate', body)
export const chooseUin = (body) => send('account.ChooseUin', body)
export const loginThird = (body) => send('account.LoginThird', body)

/**
 * 注册
 * @param body.way 登录方式 现在只有'wechat_web'
 * @param body.userInfo 直接使用loginThird的返回值即可
 */
export const registerThird = (body: {
  way: string
  user_info: any
  issuer: string
}) => send('account.RegisterThird', body) as any
/** 切换团队 */
export const switchLogin = (body: {
  login_way: number
  uin: number
}): Promise<{
  jwt_token: string
}> => send('account.SwitchLogin', body) as any

// 获取个人中心详情
export function DetailPersonalCenter() {
  return send('account.DetailPersonalCenter', {})
}

// 上传头像
export const uploadAvatarImg = (
  data: { file: File; purpose: string } | FormData,
  config?: any,
) => upload('account.UploadCustomerImage', data, config)

// 更新个人信息
export function UpdatePersonalInfo(data: {
  avatar_url: string
  name: string
  email: string
}) {
  return send('account.UpdateUserInfo', data)
}

// 获取验证码
export const getVerifyCode = (data: { phone: string }) =>
  send('account.UpdatePhoneSendCode', data)

// 修改手机号
export const updatePhone = (data: { phone: string; phone_code: string }) =>
  send('account.UpdatePhoneVerifyCode', data)

// 修改密码
export const UpdateAccountPassword = (data: {
  old_password: string
  new_password: string
}) => send('account.UpdateAccountPassword', data)

// 登录页修改密码接口
export const updatePasswordWithRefreshToken = (data: {
  old_password: string
  new_password: string
  user_id: number
  refresh_token: string
}) => send('account.ChangeDefaultPassword', data)

// 忘记密码
export const forgotPassword = (data: {
  phone: string
  code: string
  password: string
}) => send('account.ForgotPassword', data)

// 忘记密码-发送验证码
export const sendVerifyCodeForForgot = (data: { phone: string; key: string }) =>
  send('account.RequestPasswordResetCode', data)

// 微信登录
export const wxLogin = (body: any) => send('admin.LoginEmployeeWechat', body)

// 绑定微信
export const bindWx = (body: any) => send('admin.BindEmployeeWechat', body)

// 换绑微信
export const changeWx = (body: any) => send('admin.ChangeMyWechat', body)

/** 获取用户全部uin */
export const getAllUin = () => send('account.ListUin', {})

export const approveCliAuth = (user_code: string) =>
  send('account.CLIAuthApprove', { user_code })

export const denyCliAuth = (user_code: string) =>
  send('account.CLIAuthDeny', { user_code })

export const getCliAuthInfo = (user_code: string) =>
  send('keapi.CLIAuthInfo', { user_code })

/** 新版本换绑微信 */
export const changeWX_account = (body: { code: string }) =>
  send('account.BindUserWechat', body)

/** 校验密码 */
export const checkPassword = (data: { password: string }) =>
  send('account.CheckPassword', data)

/** 获取公司管理员信息 */
export const getCompanyAdmins = () => send('account.GetCompanyAdmins', {})

/** 获取人事树信息 */
export const getPersonnelInfo = (data: { include_employee: boolean }) =>
  send('account.GetDepartmentTree', data)

/** 创建部门 */
export const createDepartment = (data: { name: string; parent_id: number }) =>
  send('account.CreateDepartment', data)

/** 新增部门员工 */
export const createDepartmentEmployee = (data: {
  employee: {
    department_ids: number[]
    email?: string
    name: string
    phone?: string
    sys_role: string
  }
}) => send('account.CreateDepartmentEmployee', data)

/** 创建组织员工(私有化版) */
export const createDepartmentEmployeePrivate = (data: {
  employee: {
    department_ids: number[]
    email?: string
    name: string
    phone?: string
    sys_role: string
  }
}) => send('account.CreateDepartmentEmployeePrivate', data)

/** 删除部门 */
export const deleteDepartment = (data: { id: number }) =>
  send('account.DeleteDepartment', data)

/** 编辑部门员工 增量更新*/
export const editDepartmentEmployee = (data: {
  employee: {
    uin: number
    employee_id: number
    department_ids?: number[]
    email?: string
    name?: string
    phone?: string
    sys_role?: 'sys_admin'
  }
}) => send('account.EditDepartmentEmployee', data)

/** 编辑组织员工(私有化版) */
export const editDepartmentEmployeePrivate = (data: {
  employee: {
    uin: number
    employee_id: number
    department_ids?: number[]
    email?: string
    name?: string
    phone?: string
    sys_role?: 'sys_admin'
  }
}) => send('account.EditDepartmentEmployeePrivate', data)

/** 移动部门 */
export const moveDepartment = (data: {
  /** 要移动的部门 */
  department_id: number
  /** 目的地的前一个id 如果在顶部则是0 */
  pre_id: number
  /** 目的地的后一个id 如果在底部则是0 */
  post_id: number
}) => send('account.MoveDepartment', data)

/** 重命名部门 */
export const renameDepartment = (data: { id: number; name: string }) =>
  send('account.RenameDepartment', data)

/** 设置密码修改提醒状态 */
export const setPasswordChangeReminder = (data: {
  always_ignore: boolean
  user_id: number
  refresh_token: string
}) => send('account.ChangePasswordNotice', data)

/** 重置密码 */
export const resetPassword = (data: { uin: number }) =>
  send('account.ResetPassword', data)

/** 更新网站信息 */
export const updateWebsiteInfo = (data: {
  website_info: {
    website_logo: string
    website_name: string
  }
}) => send('account.UpdateWebsiteInfo', data)
