import service from '@/utils/request'

export const getCurrentUser = () => service.get('/user/info')
export const logout = () => service.post('/auth/logout')
