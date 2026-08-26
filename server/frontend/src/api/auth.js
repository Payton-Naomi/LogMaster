import service from '@/utils/request'

export const getCurrentUser = () => service.get('/user/info')
export const logout = () => service.post('/auth/logout')
export const externalLogin = (payload) => service.post('/auth/external/login', payload, { skipAuthRedirect: true })
export const externalRegister = (payload) => service.post('/auth/external/register', payload, { skipAuthRedirect: true })
export const externalPasswordReset = (payload) => service.post('/auth/external/password-reset', payload, { skipAuthRedirect: true })
