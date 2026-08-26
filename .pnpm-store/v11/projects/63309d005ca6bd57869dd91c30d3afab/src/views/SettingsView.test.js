import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SettingsView from './SettingsView.vue'

describe('SettingsView defaults', () => {
  it('renders complete values before the backend responds', async () => {
    const wrapper = mount(SettingsView, {
      props: { settings: {}, invoke: vi.fn().mockRejectedValue(new Error('offline')) },
      global: { stubs: { Tooltip: { template: '<div><slot /></div>' }, AntImage: true } },
    })

    await Promise.resolve()
    const values = wrapper.findAll('input').map((input) => input.element.value)
    expect(values).toContain('D:\\LogMaster\\LocalLog')
    expect(values).toContain('1800')
    expect(values).toContain('128')
    expect(values).toContain('100000')
    expect(values).toContain('50')
    expect(values).toContain('80')
    expect(values).toContain('http://localhost:8080/api')
    expect(values).toContain('300')
    expect(values).toContain('5')
  })
})
