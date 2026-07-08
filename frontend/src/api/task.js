import service from '@/utils/request'
import { useMock } from '@/utils/mock'
import { mockGetTasks, mockGetTaskDetail, mockDeleteTask, mockTaskResults } from '@/utils/mock'

export const getTasks = (params) => {
  if (useMock()) return mockGetTasks(params)
  return service.get('/tasks', { params })
}

export const getTaskDetail = (taskId) => {
  if (useMock()) return mockGetTaskDetail(taskId)
  return service.get(`/tasks/${taskId}`)
}

export const deleteTask = (taskId) => {
  if (useMock()) return mockDeleteTask(taskId)
  return service.delete(`/tasks/${taskId}`)
}

export const getTaskResults = (taskId) => {
  if (useMock()) return mockTaskResults(taskId)
  return service.get(`/tasks/${taskId}/results`)
}

export const getDashboardStats = () => {
  if (useMock()) {
    const { mockDashboardStats } = require('@/utils/mock')
    return mockDashboardStats()
  }
  return service.get('/dashboard/stats')
}