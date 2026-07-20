import service from '@/utils/request'

export const getScenarios = () => service.get('/scenarios')
export const createScenario = (scenario) => service.post('/scenarios', scenario)
export const updateScenario = (id, scenario) => service.put(`/scenarios/${id}`, scenario)
export const deleteScenario = (id) => service.delete(`/scenarios/${id}`)
