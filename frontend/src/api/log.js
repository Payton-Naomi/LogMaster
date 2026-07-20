import service from '@/utils/request'

export const uploadLog = (file, projectName, version) => {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('project_name', projectName)
  formData.append('version', version || '')
  return service.post('/logs/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export const uploadLogs = (files, projectName, version) => {
  const formData = new FormData()
  files.forEach((file) => formData.append('file', file))
  formData.append('project_name', projectName)
  formData.append('version', version || '')
  return service.post('/logs/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
}

export const getLogs = (params) => service.get('/logs', { params })
export const getLogDetail = (uploadId) => service.get(`/logs/${uploadId}`)
export const getProjects = () => service.get('/projects')
export const getComPorts = () => service.get('/system/com-ports')
export const inspectLog = (file) => {
  const formData = new FormData()
  formData.append('file', file)
  return service.post('/logs/inspect', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
}
export const startSerialCollect = (comPort, projectName) => service.post('/logs/serial/start', { com_port: comPort, project_name: projectName })
export const stopSerialCollect = (taskId) => service.post('/logs/serial/stop', { task_id: taskId })
