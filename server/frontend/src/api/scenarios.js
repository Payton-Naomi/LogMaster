import service from '@/utils/request'

export const getScenarios = () => service.get('/scenarios')
export const getScenario = (id) => service.get(`/scenarios/${id}`)
export const createScenario = (scenario) => service.post('/scenarios', scenario)
export const updateScenario = (id, scenario) => service.put(`/scenarios/${id}`, scenario)
export const setScenarioEnabled = (id, enabled) => service.patch(`/scenarios/${id}/enabled`, { enabled })
export const deleteScenario = (id) => service.delete(`/scenarios/${id}`)
