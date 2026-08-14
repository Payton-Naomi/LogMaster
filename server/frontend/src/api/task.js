import service from '@/utils/request'

export const getTasks = (params) => service.get('/tasks', { params })
export const getTaskDetail = (taskId) => service.get(`/tasks/${taskId}`)
export const deleteTask = (taskId) => service.delete(`/tasks/${taskId}`)
export const getTaskResults = (taskId, params) => service.get(`/tasks/${taskId}/results`, { params })
export const getAgentResults = (taskId) => service.get(`/tasks/${taskId}/agent-results`)
export const getDashboardStats = (days = 7) => service.get('/dashboard/stats', { params: { days } })
