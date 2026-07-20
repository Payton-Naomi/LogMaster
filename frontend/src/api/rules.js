import service from '@/utils/request'

export const getRules = () => service.get('/rules')
export const addRule = (rule) => service.post('/rules', rule)
export const updateRule = (id, rule) => service.put(`/rules/${id}`, rule)
export const deleteRule = (id) => service.delete(`/rules/${id}`)
