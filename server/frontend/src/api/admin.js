import service from '@/utils/request'

const adminRequest = { skipAuthRedirect: true }

export const getAdminUsers = () => service.get('/admin/users', adminRequest)
export const updateAdminUserRole = (id, role) => service.put(`/admin/users/${id}/role`, { role }, adminRequest)
export const restoreAdminUserRole = (id) => service.put(`/admin/users/${id}/restore-feishu-role`, {}, adminRequest)
export const getAdminProjects = () => service.get('/admin/projects', adminRequest)
export const createAdminProject = (project) => service.post('/admin/projects', project, adminRequest)
export const updateAdminProject = (id, project) => service.put(`/admin/projects/${id}`, project, adminRequest)
export const deleteAdminProject = (id) => service.delete(`/admin/projects/${id}`, adminRequest)
export const getProjectOptions = () => service.get('/admin/project-options', adminRequest)
export const createProjectOption = (option) => service.post('/admin/project-options', option, adminRequest)
export const updateProjectOption = (id, option) => service.put(`/admin/project-options/${id}`, option, adminRequest)
export const deleteProjectOption = (id) => service.delete(`/admin/project-options/${id}`, adminRequest)
export const getUploadCapacity = () => service.get('/admin/upload-capacity', adminRequest)
export const updateUploadCapacity = (capacity) => service.put('/admin/upload-capacity', capacity, adminRequest)
export const getAdminKeywordRules = () => service.get('/admin/keyword-rules', adminRequest)
export const deleteAdminKeywordRule = (id) => service.delete(`/admin/keyword-rules/${id}`, adminRequest)
export const getPermissionRequests = () => service.get('/admin/permission-requests', adminRequest)
export const createPermissionRequest = (request) => service.post('/admin/permission-requests', request, adminRequest)
export const cancelPermissionRequest = (id) => service.delete(`/admin/permission-requests/${id}`, adminRequest)
export const decidePermissionRequest = (id, decision) => service.put(`/admin/permission-requests/${id}/decision`, decision, adminRequest)
export const getRuntimeLogs = () => service.get('/admin/runtime-logs', adminRequest)
export const getProjectRequests = () => service.get('/admin/project-requests', adminRequest)
export const createProjectRequest = (request) => service.post('/admin/project-requests', request, adminRequest)
export const decideProjectRequest = (id, decision) => service.put(`/admin/project-requests/${id}/decision`, decision, adminRequest)
export const importAdminKeywordRules = (file, defaults) => {
  const formData = new FormData()
  formData.append('file', file)
  Object.entries(defaults).forEach(([key, value]) => formData.append(key, value ?? ''))
  return service.post('/admin/keyword-rules/import', formData, {
    ...adminRequest,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}
