import service from '@/utils/request'
import { useMock } from '@/utils/mock'
import { mockGetRules, mockAddRule, mockDeleteRule } from '@/utils/mock'

export const getRules = () => {
  if (useMock()) return mockGetRules()
  return service.get('/rules')
}

export const addRule = (keyword, level) => {
  if (useMock()) return mockAddRule(keyword, level)
  return service.post('/rules', { keyword, level })
}

export const deleteRule = (id) => {
  if (useMock()) return mockDeleteRule(id)
  return service.delete(`/rules/${id}`)
}