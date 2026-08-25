import service from '@/utils/request'

export const updateResultStatus = (id, status) => service.patch(`/results/${id}/status`, { status })
export const getResultComments = (id) => service.get(`/results/${id}/comments`)
export const addResultComment = (id, comment) => service.post(`/results/${id}/comments`, comment)
export const assignResult = (id, assignedTo) => service.put(`/results/${id}/assignment`, { assigned_to: assignedTo || '' })
export const getResultHistory = (id) => service.get(`/results/${id}/history`)
export const batchAssignResults = (resultIds, assignedTo) => service.put('/results/batch/assignment', { result_ids: resultIds, assigned_to: assignedTo || '' })
