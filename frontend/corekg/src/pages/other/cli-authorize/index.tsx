import React, { useEffect, useMemo, useState } from 'react'
import { App, Button, Card, Select, Spin, Typography } from 'antd'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  approveCliAuth,
  denyCliAuth,
  getCliAuthInfo,
  switchLogin,
} from '@/api/account'
import useLocalStore from '@/stores/local'

const CLIAuthPage: React.FC = () => {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const token = useLocalStore((state) => state.token)
  const userInfo = useLocalStore((state) => state.userInfo)
  const uinList = useLocalStore((state) => state.uinList)
  const setLogin = useLocalStore((state) => state.setLogin)
  const userCode = searchParams.get('user_code') || ''
  const [clientInfo, setClientInfo] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [completed, setCompleted] = useState('')

  const returnTo = `${location.pathname}${location.search}`
  const companyUins = useMemo(
    () => uinList.filter((item) => item.subjectType !== 'individual'),
    [uinList],
  )
  const currentUin = companyUins.find(
    (item) => String(item.id) === String(userInfo.uinId),
  )

  useEffect(() => {
    if (!userCode) {
      setLoading(false)
      return
    }
    if (!token) {
      navigate(`/login?return_to=${encodeURIComponent(returnTo)}`, {
        replace: true,
      })
      return
    }
    getCliAuthInfo(userCode)
      .then((value) => setClientInfo(value))
      .catch(() => message.error('授权码不存在或已过期'))
      .finally(() => setLoading(false))
  }, [message, navigate, returnTo, token, userCode])

  const changeOrganization = async (value: string) => {
    const target = companyUins.find((item) => String(item.id) === value)
    if (!target) return
    try {
      const result = await switchLogin({
        login_way: userInfo.loginWay || 0,
        uin: Number(target.id),
      })
      setLogin({
        token: result.jwt_token,
        userInfo: { ...userInfo, uinId: target.id },
      })
    } catch (error) {
      message.error('切换组织失败')
    }
  }

  const submit = async (action: 'approve' | 'deny') => {
    setSubmitting(true)
    try {
      if (action === 'approve') {
        await approveCliAuth(userCode)
        message.success('已授权 CoreKG CLI，可以返回终端继续')
      } else {
        await denyCliAuth(userCode)
        message.info('已拒绝本次 CLI 授权')
      }
      setCompleted(action)
    } catch (error) {
      message.error(action === 'approve' ? '授权失败或授权码已过期' : '拒绝授权失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return <Spin className='w-full mt-20' />
  }

  if (!userCode || !clientInfo) {
    return (
      <div className='w-full min-h-screen flex items-center justify-center'>
        <Card title='CoreKG CLI 授权' className='w-[32rem]'>
          <Typography.Text type='danger'>授权码不存在或已过期。</Typography.Text>
        </Card>
      </div>
    )
  }

  return (
    <div className='w-full min-h-screen flex items-center justify-center bg-gray-50'>
      <Card title='授权 CoreKG CLI' className='w-[32rem]'>
        <div className='flex flex-col gap-4'>
          <Typography.Paragraph>
            终端程序 <Typography.Text strong>{clientInfo.client_name || 'corekg-cli'}</Typography.Text>{' '}
            正在请求访问你的 CoreKG 知识库。
          </Typography.Paragraph>
          <Typography.Text type='secondary'>授权码：{userCode}</Typography.Text>
          <Select
            value={currentUin ? String(currentUin.id) : undefined}
            placeholder='选择组织'
            options={companyUins.map((item) => ({
              label: item.companyName || item.uinName,
              value: String(item.id),
            }))}
            onChange={changeOrganization}
            disabled={Boolean(completed) || submitting}
          />
          {currentUin && (
            <Typography.Text>
              当前组织：{currentUin.companyName || currentUin.uinName}
            </Typography.Text>
          )}
          {!currentUin && (
            <Typography.Text type='danger'>当前账户没有可授权的组织身份。</Typography.Text>
          )}
          <div className='flex gap-3 justify-end'>
            <Button
              onClick={() => submit('deny')}
              disabled={Boolean(completed) || submitting}
            >
              拒绝
            </Button>
            <Button
              type='primary'
              onClick={() => submit('approve')}
              disabled={Boolean(completed) || submitting || !currentUin}
              loading={submitting}
            >
              允许访问
            </Button>
          </div>
          {completed && (
            <Typography.Text type='success'>
              {completed === 'approve'
                ? '授权已完成，请返回终端。'
                : '授权已拒绝，可以关闭此页面。'}
            </Typography.Text>
          )}
        </div>
      </Card>
    </div>
  )
}

export default CLIAuthPage
