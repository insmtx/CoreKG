import axios from 'axios'
import globalConfig from '@/config'
import useLocalStore from '@/stores/local'
import { showRequestErrorOnce } from '@/utils/notification'

const { withPrefix } = globalConfig
const request = axios.create({
  baseURL: globalConfig.apiUrl,
  timeout: 600000000,
  headers: {
    Env: globalConfig.apiEnv,
    'Content-Type': 'application/json',
    'Accept-Language': window.__LANG,
  },
})

const whiteList = [
  'account.LoginByPassword',
  'account.GetLoginSetting',
  'account.ChooseUin',
  'account.LoginThird',
  'account.RegisterThird',
  'account.GetInviteInfo',
  'account.BindCompanyWithPermSet',
  'account.LoginByPasswordPrivate',
  'status.GetClusterID',
  'corekg.GetLicenseInfo',
  'account.ChangePasswordNotice',
  'account.ChangeDefaultPassword',
  'NowDeployMode',
  'SwitchPrivateEvn',
  'account.ForgotPassword',
  'account.RequestPasswordResetCode',
  'account.GetGlobalInfo',
  'keapi.CLIAuthInfo',
  // 联系售前表单接口，/version 为公开页面，无需 token
  'forest.VersionUpgradeSendCode',
  'forest.VersionUpgradeVerify',
]

// 请求拦截器
request.interceptors.request.use(
  (config) => {
    if (whiteList.some((u) => config.url?.includes(u))) {
      return config
    }

    // account.CreateCompany 特殊处理：如果传了 refresh_token 参数，则不添加请求头 token（登录页面创建）
    // 如果没传 refresh_token 参数，则添加请求头 token（切换组织弹窗创建）
    if (config.url?.includes('account.CreateCompany')) {
      const requestData = (config.data as any)?.request
      if (requestData?.refresh_token) {
        // 登录页面创建，传了 refresh_token，不添加请求头 token
        return config
      }
      // 切换组织弹窗创建，没传 refresh_token，添加请求头 token
      const token = useLocalStore.getState().token
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      } else {
        toLogin()
        return Promise.reject(new Error('not login'))
      }
      return config
    }

    const token = useLocalStore.getState().token
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    } else {
      toLogin()
      return Promise.reject(new Error('not login'))
    }

    return config
  },
  (error) => {
    return Promise.reject(new Error(error))
  },
)

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    const { data, config } = response

    // 特殊处理 getClusterID 接口，该接口直接返回字符串
    if (config.url && config.url.includes('status.GetClusterID')) {
      return data
    }

    if (data.code === 0) {
      return data.response || data.Response || data
    } else if (data.code === 20001) {
      // 文件解析失败
      return
    } else if (data.code === 20002) {
      return data
    } else if (data.code === 401) {
      toLogin()
    } else {
      const msg = data.message || '调用接口失败'
      showRequestErrorOnce(msg)
      console.log('defined: api response error: ', msg)
      return Promise.reject(new Error(msg))
    }
  },
  (error) => {
    if (axios.isCancel(error) || error.code === 'ERR_CANCELED') {
      return Promise.reject(error)
    }

    let msg = ''
    if (error.response) {
      if (error.response.status === 401) {
        // 未登陆
        toLogin()
        return
      }
      if (error.response?.data?.message) {
        msg = error.response.data.message
      } else {
        msg = error.response.data
      }
    } else {
      msg =
        error.message === 'Network Error'
          ? '网络未连接，请检查后重试'
          : error.message
    }

    if (!['not login', 'workflow_test_run_error'].includes(msg)) {
      showRequestErrorOnce(msg)
    }
    console.log('undefined: api response error: ', msg)
    return Promise.reject(msg)
  },
)

export const toLogin = () => {
  showRequestErrorOnce('登录信息已过期，请重新登录')
  useLocalStore.getState().setLogout()
}

export const send = (url: string, data: any, config?: any): Promise<any> => {
  const body = {
    cmd: withPrefix(url),
    env: globalConfig.apiEnv,
    version: globalConfig.version,
    request: data,
  }
  return request.post(withPrefix(url), body, config)
}
export const send2 = (url: string, data: any): Promise<any> => {
  return request.get(withPrefix(url), { params: data })
}
export const download = (
  url: string,
  data: any,
  defaultFilename?: string,
): Promise<any> => {
  const body = {
    cmd: url,
    env: globalConfig.apiEnv,
    version: globalConfig.version,
    request: data,
  }
  return axios({
    url: globalConfig.apiUrl + url,
    method: 'POST',
    responseType: 'blob', // 重要：将响应类型设置为 blob
    headers: {
      Env: globalConfig.apiEnv,
    },
    data: body,
  })
    .then((response) => {
      let filename = defaultFilename
      const contentDisposition = response.headers['content-disposition']
      if (contentDisposition) {
        const filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/
        const matches = filenameRegex.exec(contentDisposition)
        if (matches != null && matches[1]) {
          filename = matches[1].replace(/['"]/g, '')
          // 处理可能的 URL 编码
          filename = decodeURIComponent(filename)
        }
      }

      const blob = new Blob([response.data])
      const downloadUrl = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = downloadUrl
      link.download = filename!
      document.body.appendChild(link)
      link.click()
      link.remove()
      // 清理并释放 URL 对象
      window.URL.revokeObjectURL(downloadUrl)
    })
    .catch((error) => {
      const errorMsg =
        error.message === 'Network Error'
          ? '网络未连接，请检查后重试'
          : error.message
      console.error('下载文件时出错:', errorMsg)
      throw new Error(errorMsg)
    })
}

export const _download = (url: string, query: any) => {
  return request.get(url, { params: query, responseType: 'blob' })
}

export const upload = (url: string, data: any, currentConfig: any) => {
  const formData = new FormData()
  for (const key in data) {
    formData.append(key, data[key])
  }
  return request.post(withPrefix(url), formData, currentConfig)
}

// coze iframe
export const getExternalStatus = ({
  space_id,
  bot_id,
}: {
  space_id: string
  bot_id: string
}): Promise<any> => {
  return request.post('/coze/api/internal/agent/external_info', {
    space_id,
    bot_id,
  })
}

export const setExternalStatus = ({
  space_id,
  bot_id,
  status,
}: {
  space_id: string
  bot_id: string
  status: string
}): Promise<any> => {
  return request.post('/coze/api/internal/agent/set_external_status', {
    space_id,
    bot_id,
    status,
  })
}
