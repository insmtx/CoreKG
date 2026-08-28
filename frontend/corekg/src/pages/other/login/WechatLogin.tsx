import React, { useEffect, useState } from 'react'
import { useMount } from 'ahooks'
import { useTranslation } from 'react-i18next'
import { cn } from '@/utils'
import { Agreement } from '@/components/agreement'
import LoadingCover from '@/components/common/LoadingCover'
import { loadWxLoginScript, getWxLoginCode } from '@/utils/wx'

declare global {
  interface Window {
    WxLogin: any
  }
}

interface WechatLoginProps {
  appId: string
  returnTo?: string
}

const WechatLogin: React.FC<WechatLoginProps> = ({ appId, returnTo }) => {
  const [codeLoading, setCodeLoading] = useState(true)
  const { t } = useTranslation('pages')
  useMount(() => {
    initWechatLogin()
  }, [])

  const initWechatLogin = async () => {
    try {
      setCodeLoading(true)
      await loadWxLoginScript()
      const callback = returnTo
        ? `/callback?return_to=${encodeURIComponent(returnTo)}`
        : '/callback'
      await getWxLoginCode(appId, callback)
      setTimeout(() => {
        const qrcodeContainer = document.querySelector('#wxQrcode')
        const iframe = qrcodeContainer.getElementsByTagName('iframe')[0]
        if (iframe) {
          iframe.onload = () => {
            setCodeLoading(false)
          }
        } else {
          setCodeLoading(false)
        }
      }, 100)
    } catch (error) {
      console.error('加载微信登录失败:', error)
    }
  }

  return (
    <div className='w-full h-60 flex flex-col items-center justify-between relative'>
      <div
        id='wxQrcode'
        className={cn('w-50 h-50', codeLoading && 'opacity-0')}
      ></div>
      <Agreement />
      <div className='text-sm text-gray-500 text-center'>
        {t('invite.noAccountScanQrCodeToRegisterImmediately')}
      </div>

      <LoadingCover loading={codeLoading} />
    </div>
  )
}

export default WechatLogin
