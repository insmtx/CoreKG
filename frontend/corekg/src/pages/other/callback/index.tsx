import React, { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { App } from 'antd'
import { useMount } from 'ahooks'
import { useTranslation } from 'react-i18next'
import { getLoginConfig, loginThird, registerThird } from '@/api/account'
import useLocalStore from '@/stores/local'
import ChooseUin from '../login/ChooseUin'
import CreateCompany from '../login/CreateCompany'

const Callback: React.FC = () => {
  const { t } = useTranslation('pages')
  const { t: tC } = useTranslation('common')
  const { t: tM } = useTranslation('messages')
  const localStore = useLocalStore()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [searchParams] = useSearchParams()
  const requestedReturnTo = searchParams.get('return_to')
  const returnTo = requestedReturnTo?.startsWith('/cli/authorize')
    ? requestedReturnTo
    : '/'
  const loginPath =
    returnTo === '/'
      ? '/login'
      : `/login?return_to=${encodeURIComponent(returnTo)}`

  const [loginConfig, setLoginConfig] = useState({
    bg: '',
    title: '',
    loginTypes: [],
  })

  const [modelType, setModelType] = useState('chooseUin') // chooseUin createCompany
  const [chooseUinInfo, setChooseUinInfo] = useState({
    refreshToken: '',
    userInfo: {},
    uinList: [],
    username: '', // 微信登录没有账号密码
    password: '',
    companyQuota: undefined as number | undefined,
  })

  const handleLoginSuccess = () => {
    navigate(returnTo)
  }

  const [initLoading, setInitLoading] = useState(true)

  const init = async () => {
    const code = searchParams.get('code')
    const state = searchParams.get('state')

    if (!code || !state) {
      message.error(tM('loginParameterError'))
      setInitLoading(false)
      return
    }

    // 先获取登录配置，确保背景图片能正确显示
    try {
      const loginConfigRes = await getLoginConfig({
        domain_name: location.origin,
        // domain_name: 'https://example.com', //为了本地开发登录页面，临时设置的，开发完之后需要注释掉这行代码，恢复上一行的代码
      })
      setLoginConfig({
        bg: loginConfigRes.image_url,
        title: loginConfigRes.title,
        loginTypes: [],
      })
    } catch (error) {
      console.log('获取登录配置失败:', error)
    }

    const body = {
      code: code,
      way: 'wechat_web',
      domain_name: location.origin,
      // domain_name: 'https://example.com', //为了本地开发登录页面，临时设置的，开发完之后需要注释掉这行代码，恢复上一行的代码
    }
    try {
      let res: any = await loginThird(body)
      if (res.login_status === 'failed') {
        message.error(tM('loginFailedPleaseRelogin'))
        setInitLoading(false)
        setTimeout(() => {
          navigate(loginPath)
        }, 1500)
        return
      }
      if (res.login_status === 'register') {
        if (res.allow_register) {
          try {
            const loginConfig = await getLoginConfig({
              domain_name: location.origin,
            })
            const registerRes = await registerThird({
              way: 'wechat_web',
              user_info: res.user_info,
              issuer: loginConfig.issuer,
            })
            res = registerRes
          } catch (error) {
            setInitLoading(false)
            navigate(loginPath)
            return
          }
        } else {
          message.error(tM('userHasNoIdentityPleaseContactAdminToAdd'))
          setInitLoading(false)
          setTimeout(() => {
          navigate(loginPath)
          }, 1500)
          return
        }
      }

      // 检查必要的字段是否存在
      if (!res.user_info) {
        setInitLoading(false)
        navigate(loginPath)
        return
      }

      const userInfo = {
        id: res.user_id,
        avatar: res.user_info.avatar_url,
        name: res.user_info.name,
        uinId: res.user_info.uin,
        loginWay: res.login_way,
      }
      // 修复：对 res.uin 进行空值检查，如果不存在则使用空数组
      const uinList = (res.uin || []).map((x: any) => {
        return {
          id: x.uin.ID,
          role: x.role,
          uinName: x.uin.Name,
          companyName: x.company_name,
          companyStatus: x.company_status, // passed
          uinStatus: x.uin.UinStatus, // normal
          subjectType: x.uin.SubjectType, // company, individual (企业，个人)
          subjectId: x.uin.SubjectID, // 企业id，个人为0
          logo: x.company_logo,
          companyUserId: x.company_user_id,
        }
      })
      // 无论有多少个组织，都进入选择身份页面（新用户可以创建组织）
      // 如果没有组织，uinList 为空数组，ChooseUin 组件会显示空列表和"创建新组织"按钮
      setChooseUinInfo({
        refreshToken: res.refresh_token,
        userInfo: userInfo,
        uinList: uinList,
        username: '', // 微信登录没有账号密码
        password: '',
        companyQuota: res.user_info.company_quota, // 保存组织配额
      })
      setInitLoading(false)
    } catch (error) {
      console.log('login error', error)
      setInitLoading(false)
      navigate(loginPath)
    }
  }

  useMount(() => {
    init()
  })

  return (
    <>
      {initLoading ? (
        <div className='w-full h-screen flex items-center justify-center'>
          <span>{tC('status.loading')}</span>
        </div>
      ) : (
        <div
          className='w-full h-screen flex bg-gray-50'
          style={{
            background: `url(${loginConfig.bg}) no-repeat center center / cover`,
          }}
        >
          <div className='flex-1 h-full'>&nbsp;</div>
          <div className='flex-1 h-full flex items-center justify-center'>
            <div className='w-100 p-10 bg-white rounded-lg shadow-md'>
              <div className='text-center'>
                <h2 className='text-3xl font-bold text-gray-900 mb-2'>
                  {modelType === 'chooseUin' &&
                    t('other.pleaseSelectYourIdentity')}
                  {modelType === 'createCompany' &&
                    t('other.createOrganization')}
                </h2>
              </div>
              {modelType === 'chooseUin' && (
                <div className='w-full h-80 relative'>
                  <ChooseUin
                    info={chooseUinInfo}
                    onLogin={handleLoginSuccess}
                    onCreateClick={() => setModelType('createCompany')}
                  />
                </div>
              )}
              {modelType === 'createCompany' && (
                <div className='w-full h-80 relative'>
                  <CreateCompany
                    info={chooseUinInfo}
                    onLogin={handleLoginSuccess}
                  />
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
}

export default Callback
