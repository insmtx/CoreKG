import React, { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Tabs, TabsProps } from 'antd'
import { useMount } from 'ahooks'
import { useTranslation } from 'react-i18next'
import { cn } from '@/utils'
import { getLoginConfig } from '@/api/account'
import LoadingCover from '@/components/common/LoadingCover'
import ChooseUin from './ChooseUin'
import CreateCompany from './CreateCompany'
import PasswordLogin from './PasswordLogin'
import WechatLogin from './WechatLogin'
import styles from './styles.module.scss'

interface LoginConfig {
  bg: string
  title: string
  loginTypes: any[]
}

const LoginPage: React.FC = () => {
  const { t } = useTranslation('pages')
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const requestedReturnTo = searchParams.get('return_to')
  const returnTo = requestedReturnTo?.startsWith('/cli/authorize')
    ? requestedReturnTo
    : '/'

  const [activeTab, setActiveTab] = useState('wechat')
  const [tabItems, setTabItems] = useState<TabsProps['items']>([])
  const [loginConfig, setLoginConfig] = useState<LoginConfig>({
    bg: '',
    title: '',
    loginTypes: [],
  })

  const [modelType, setModelType] = useState('login') // login chooseUin createCompany
  const [chooseUinInfo, setChooseUinInfo] = useState({
    refreshToken: '',
    userInfo: {},
    uinList: [],
    username: '',
    password: '',
    companyQuota: undefined as number | undefined,
  })
  const handleChooseUin = (uin: any) => {
    setModelType('chooseUin')
    setChooseUinInfo({
      refreshToken: uin.refreshToken,
      userInfo: uin.userInfo,
      uinList: uin.uinList,
      username: uin.username || '',
      password: uin.password || '',
      companyQuota: uin.companyQuota,
    })
  }

  const handleLoginSuccess = () => {
    navigate(returnTo)
  }

  const [initLoading, setInitLoading] = useState(true)
  const init = async () => {
    try {
      const res = await getLoginConfig({
        domain_name: location.origin,
        // domain_name: 'https://example.com', //为了本地开发登录页面，临时设置的，开发完之后需要注释掉这行代码，恢复上一行的代码
      })
      const loginTypes: TabsProps['items'] = []
      if (res.wechat.enable) {
        loginTypes.push({
          key: 'wechat',
          label: <span>{t('other.wechatLogin')}</span>,
          children: (
            <WechatLogin
              appId={res.wechat.appid}
              returnTo={returnTo}
              // onLogin={handleLoginSuccess} // 微信登录成功后通过回调页面 /callback 处理，不需要 onLogin 回调
            />
          ),
        })
      }
      if (res.password.enable) {
        loginTypes.push({
          key: 'password',
          label: <span>{t('other.accountPasswordLogin')}</span>,
          children: (
            <PasswordLogin
              onChooseUin={handleChooseUin}
              onLogin={handleLoginSuccess}
            />
          ),
        })
      }

      setTabItems(loginTypes)
      if (loginTypes.length > 0) {
        setActiveTab(loginTypes[0].key)
      }
      setLoginConfig({
        bg: res.image_url,
        title: res.title,
        loginTypes: res.login_ways || [],
      })
      console.log('login config', res)
    } finally {
      setInitLoading(false)
    }
  }

  useMount(() => {
    init()
  })

  return (
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
              {modelType === 'login' && t('other.welcomeToLogin')}
              {modelType === 'chooseUin' && t('other.pleaseSelectYourTeam')}
              {modelType === 'createCompany' && t('other.createOrganization')}
            </h2>
          </div>
          {modelType === 'login' && (
            <div className='w-full h-90 relative'>
              <Tabs
                activeKey={activeTab}
                onChange={setActiveTab}
                centered
                items={tabItems}
                className={cn('login-tabs', styles['login-tabs'])}
              />

              <LoadingCover loading={initLoading} />
            </div>
          )}
          {modelType === 'chooseUin' && (
            <div className='w-full h-90 relative'>
              <ChooseUin
                info={chooseUinInfo}
                onLogin={handleLoginSuccess}
                onCreateClick={() => setModelType('createCompany')}
              />
            </div>
          )}
          {modelType === 'createCompany' && (
            <div className='w-full h-90 relative'>
              <CreateCompany
                info={chooseUinInfo}
                onLogin={handleLoginSuccess}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default LoginPage
