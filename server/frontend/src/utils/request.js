import axios from 'axios'
import { ElMessage } from 'element-plus'

const service = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  timeout: 30000
})

// 响应拦截器
service.interceptors.response.use(
  (response) => {
    const res = response.data
    if (res.code !== 0) {
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || 'Error'))
    }
    return res.data
  },
  (error) => {
    if (error.response) {
      const { status } = error.response
      if (status === 401) {
        if (!error.config?.skipAuthRedirect) {
          ElMessage.error('登录已过期，请重新登录')
          window.location.href = import.meta.env.VITE_FEISHU_LOGIN_URL
        }
      } else if (status === 403) {
        ElMessage.error(error.response.data?.message || '当前账号没有执行此操作的权限')
      } else {
        ElMessage.error(error.response.data?.message || '服务器错误')
      }
    } else if (error.request) {
      ElMessage.error('网络连接异常')
    } else {
      ElMessage.error(error.message)
    }
    return Promise.reject(error)
  }
)

export default service
