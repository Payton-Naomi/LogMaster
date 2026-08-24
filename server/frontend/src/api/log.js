import service from '@/utils/request'

const appendUploadMetadata = (formData, metadata) => {
  Object.entries(metadata).forEach(([key, value]) => {
    if (value !== undefined && value !== null) formData.append(key, Array.isArray(value) ? JSON.stringify(value) : value)
  })
}

export const uploadLog = (file, metadata, scenarioIds = [], options = {}) => {
  const formData = new FormData()
  formData.append('file', file)
  appendUploadMetadata(formData, metadata)
  formData.append('scenario_ids', JSON.stringify(scenarioIds || []))
  return service.post('/logs/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data', 'Idempotency-Key': metadata.client_request_id },
    timeout: 0,
    ...options
  })
}

export const uploadLogs = (files, metadata, scenarioIds = [], options = {}) => {
  const formData = new FormData()
  files.forEach((file) => formData.append('file', file))
  appendUploadMetadata(formData, metadata)
  formData.append('scenario_ids', JSON.stringify(scenarioIds || []))
  return service.post('/logs/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data', 'Idempotency-Key': metadata.client_request_id },
    timeout: 0,
    ...options
  })
}

export const getLogs = (params) => service.get('/logs', { params })
export const getLogDetail = (uploadId) => service.get(`/logs/${uploadId}`)
export const getLogPreview = (uploadId, fileId) => service.get(`/logs/${uploadId}/preview`, { params: fileId ? { file_id: fileId } : {} })
export const searchLog = (uploadId, params) => service.get(`/logs/${uploadId}/search`, { params })
export const downloadLog = (uploadId, params = {}) => service.get(`/logs/${uploadId}/download`, { params, responseType: 'blob' })
export const getQueryStatus = (queryCode) => service.get(`/query/${encodeURIComponent(queryCode)}`, { skipAuthRedirect: true })
export const collectQuerySession = (queryCode) => service.post(`/query/${encodeURIComponent(queryCode)}/collect`)
export const getProjects = () => service.get('/projects')
export const getUploadConfig = () => service.get('/upload-config')
export const inspectLog = (file) => {
  const formData = new FormData()
  formData.append('file', file)
  return service.post('/logs/inspect', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
}
