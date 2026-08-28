import axios from 'axios'
import { ElMessage } from 'element-plus'

const service = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 30000
})

// 响应拦截器
service.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob' || response.config.responseType === 'arraybuffer') return response.data
    const res = response.data
    if (res.code !== 0) {
      const message = formatApiMessage(res.message)
      ElMessage.error(message)
      return Promise.reject(new Error(message || 'Error'))
    }
    return res.data
  },
  (error) => {
    if (error.response) {
      const { status } = error.response
      if (status === 401) {
        if (!error.config?.skipAuthRedirect) {
          ElMessage.error('登录已过期，请重新登录')
          window.location.href = '/login'
        }
      } else if (status === 403) {
        ElMessage.error(error.response.data?.message || '当前账号没有执行此操作的权限')
      } else {
        ElMessage.error(formatApiMessage(error.response.data?.message) || '服务器错误')
      }
    } else if (error.request) {
      ElMessage.error('网络连接异常')
    } else {
      ElMessage.error(formatApiMessage(error.message))
    }
    return Promise.reject(error)
  }
)

export default service

function formatApiMessage(message) {
  const text = String(message || '')
  if (text.includes('check keyword rule usage failed')) return '检查规则是否被测试场景引用时发生错误，请稍后重试'
  return message
}
