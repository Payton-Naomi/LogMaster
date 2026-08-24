import service from '@/utils/request'
export const getNotifications = (params) => service.get('/notifications', { params })
export const markNotificationRead = (id) => service.patch(`/notifications/${id}/read`)
export const markAllNotificationsRead = () => service.post('/notifications/read-all')
export const getNotificationSettings = () => service.get('/notification-settings')
export const updateNotificationSettings = (settings) => service.put('/notification-settings', settings)
