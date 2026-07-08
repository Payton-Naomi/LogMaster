import service from '@/utils/request'
import { useMock } from '@/utils/mock'
import { mockUpload, mockGetComPorts } from '@/utils/mock'

export const uploadLog = (file, projectName, version) => {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('project_name', projectName)
  formData.append('version', version || '1.0.0')
  if (useMock()) return mockUpload(file, projectName, version)
  return service.post('/logs/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export const getComPorts = () => {
  if (useMock()) return mockGetComPorts()
  // 若后端提供，则调用后端接口；否则使用 Web Serial API
  // 这里假设后端提供
  return service.get('/system/com-ports')
}

// 启动串口采集
export const startSerialCollect = (comPort, projectName) => {
  return service.post('/logs/serial/start', { com_port: comPort, project_name: projectName })
}

// 停止串口采集
export const stopSerialCollect = (taskId) => {
  return service.post('/logs/serial/stop', { task_id: taskId })
}