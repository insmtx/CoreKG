import version from './version'

const ENV = import.meta.env
const loginUrl = (ENV.VITE_LOGIN_URL || window.location.origin).replace(
  /\/$/,
  '',
)

if (ENV.MODE == 'development') {
  console.log('version: ', version)
}

export default {
  version,
  baseUrl: ENV.BASE_URL, // default: ''
  env: ENV.MODE, // development test production
  apiEnv: ENV.MODE === 'production' ? 'prod' : 'test',
  // apiUrl: ENV.VITE_API_URL,
  apiUrl: '',
  // apiUrl: ENV.MODE === 'development' ? ENV.VITE_API_URL : '',
  apiUrlPrefix: '/v3/',
  /** 为每个url加版本前缀 */
  withPrefix: (url: string) => {
    if (url.startsWith('account.')) {
      return `/v2/${url}`
    }
    return `/v3/${url}`
  },
  loginHub: `${loginUrl}/hub`,
  loginUrl,
}
