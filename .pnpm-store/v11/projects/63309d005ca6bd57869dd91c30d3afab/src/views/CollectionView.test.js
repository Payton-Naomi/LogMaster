import { mount, flushPromises } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import CollectionView from './CollectionView.vue'

describe('CollectionView', () => {
  it('关闭上传通道时选择停止上传需要二次确认', async () => {
    const invoke = vi.fn().mockResolvedValue(undefined)
    const device = { deviceId: 'COM24', portName: 'COM24', config: { uploadEnabled: true } }
    const desktop = {
      devices: ref([device]), selectedDeviceId: ref('COM24'), selectedDevice: ref(device),
      settings: ref({}), catalog: ref({ projects: [] }), busy: ref(false),
      connect: vi.fn(), disconnect: vi.fn(), saveDeviceConfig: vi.fn(),
      deviceLogs: () => [], withBusy: (action) => action(), invoke,
      showWarning: vi.fn(), refreshAll: vi.fn(), clearLogs: vi.fn(), sendCommand: vi.fn(),
    }
    const wrapper = mount(CollectionView, {
      attachTo: document.body,
      props: { desktop },
      global: {
        stubs: {
          ChannelList: { props: ['devices'], emits: ['toggle'], template: '<button data-test="close-port" @click="$emit(\'toggle\', devices[0], false)">关闭</button>' },
          LogConsole: true,
          ChannelInspector: true,
        },
      },
    })

    await wrapper.get('[data-test="close-port"]').trigger('click')
    await nextTick()
    expect(document.body.textContent).toContain('关闭串口，继续上传')
    expect(document.body.textContent).toContain('关闭串口，停止上传')

    const radios = document.body.querySelectorAll('input[type="radio"]')
    radios[1].click()
    await nextTick()
    const firstConfirm = [...document.body.querySelectorAll('button')].find((button) => button.textContent.includes('关闭串口'))
    firstConfirm.click()
    await nextTick()
    expect(document.body.textContent).toContain('未上传文件会保留在本机')
    expect(invoke).not.toHaveBeenCalled()

    const stopConfirm = [...document.body.querySelectorAll('button')].find((button) => button.textContent.includes('确认停止并关闭'))
    stopConfirm.click()
    await flushPromises()
    expect(invoke).toHaveBeenCalledWith('DisconnectDeviceWithUploadPolicy', 'COM24', false)
    wrapper.unmount()
  })
})
