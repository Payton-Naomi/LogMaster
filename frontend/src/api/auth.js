import service from '@/utils/request'
import { useMock } from '@/utils/mock'

// 获取当前用户信息
export const getCurrentUser = () => {
  if (useMock()) {
    return new Promise((resolve) => {
      setTimeout(() => resolve({ name: '张三', user_id: 'zhangsan' }), 200)
    })
  }
  return service.get('/auth/me')
}