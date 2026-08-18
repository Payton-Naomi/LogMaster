import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { Select, Switch } from 'ant-design-vue'
import ChannelInspector from './ChannelInspector.vue'

describe('ChannelInspector', () => {
  it('未保存通道不自动填充业务配置', () => {
    const wrapper = mount(ChannelInspector, {
      props: {
        device: { deviceId: 'COM3', configStatus: 'unconfigured', config: { deviceId: 'COM3', portName: 'COM3', configured: false, saveEnabled: true, uploadEnabled: false } },
        catalog: { projects: [{ id: 'p1', name: '项目一', versions: ['V1.0.0'], tasks: [{ id: 'normal', name: '普通挂测', type: 'normal', keywordProfiles: [{ id: 'default', name: '默认规则', rules: [{ id: 'timeout', name: '超时', match: 'timeout' }] }] }] }] },
      },
    })
    const values = wrapper.findAll('.ant-select-selection-item').map((item) => item.text())
    expect(values).not.toContain('项目一')
    expect(values).not.toContain('普通挂测')
    expect(values).not.toContain('默认规则')
    expect(wrapper.text()).toContain('选择关键字（0）')
  })

  it('项目、测试任务和关键字方案可独立选择', () => {
    const wrapper = mount(ChannelInspector, {
      props: {
        device: { deviceId: 'COM3', config: { deviceId: 'COM3', portName: 'COM3', configured: false, saveEnabled: true, uploadEnabled: true } },
        catalog: { projects: [
          { id: 'p1', name: '项目一', tasks: [{ id: 'task-one', name: '任务一' }] },
          { id: 'p2', name: '项目二', tasks: [{ id: 'task-two', name: '任务二', keywordProfiles: [{ id: 'profile-two', name: '方案二', rules: [] }] }] },
        ] },
      },
    })
    const formItem = (label) => wrapper.findAll('.ant-form-item').find((item) => item.text().includes(label))
    expect(formItem('测试任务').find('.ant-select').classes()).not.toContain('ant-select-disabled')
    expect(formItem('关键字方案').find('.ant-select').classes()).not.toContain('ant-select-disabled')
    expect(formItem('测试任务').findComponent(Select).props('options')).toEqual([
      { value: 'task-one', label: '任务一' },
      { value: 'task-two', label: '任务二' },
    ])
    expect(formItem('关键字方案').findComponent(Select).props('options')).toEqual([
      { value: 'profile-two', label: '方案二' },
    ])
  })

  it('开启云端上传后展开上传参数，未开启时隐藏', async () => {
    const wrapper = mount(ChannelInspector, {
      props: {
        device: { deviceId: 'COM3', config: { deviceId: 'COM3', portName: 'COM3', configured: false, saveEnabled: true, uploadEnabled: false, projectId: 'p1', version: 'V1.0.0', uploaderName: '   ' } },
        catalog: { projects: [{ id: 'p1', name: '项目一', versions: ['V1.0.0'] }] },
      },
    })
    expect(wrapper.text()).not.toContain('云端上传参数')
    const uploadPolicy = wrapper.findAll('.policy-item').find((item) => item.text().includes('上传云端'))
    uploadPolicy.findComponent(Switch).vm.$emit('change', true)
    await wrapper.vm.$nextTick()
    // 开关不再前置校验：直接展开上传参数，必填项在保存时由后端校验并提示
    expect(wrapper.emitted('warning')).toBeUndefined()
    expect(uploadPolicy.findComponent(Switch).props('checked')).toBe(true)
    expect(wrapper.text()).toContain('云端上传参数')
  })

  it('上传配置完整时切换保存按钮名称', () => {
    const wrapper = mount(ChannelInspector, {
      props: {
        device: { deviceId: 'COM3', config: { deviceId: 'COM3', portName: 'COM3', configured: true, saveEnabled: true, uploadEnabled: true, projectId: 'p1', version: 'V1.0.0', uploaderName: '张三', uploaderEmail: 'zhangsan@company.com' } },
        catalog: { projects: [{ id: 'p1', name: '项目一', versions: ['V1.0.0'] }] },
      },
    })
    expect(wrapper.text()).toContain('保存上传/通道配置')
		expect(wrapper.text()).toContain('上传人企业邮箱')
  })
})
