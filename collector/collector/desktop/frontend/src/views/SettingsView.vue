<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { Button, Descriptions, DescriptionsItem, Form, FormItem, Image as AntImage, Input, InputNumber, notification, Select, Switch, Tooltip } from 'ant-design-vue'
import { FolderOpenOutlined, SaveOutlined, TeamOutlined } from '@ant-design/icons-vue'
const props = defineProps({ settings: { type: Object, default: () => ({}) }, invoke: Function })
const emit = defineEmits(['saved'])
const draft = reactive({})
const state = reactive({ dirty: false, message: '' })
const storageUnit = ref('GB')
const clonePlain = (value) => JSON.parse(JSON.stringify(value || {}))
watch(() => props.settings, (value) => { Object.assign(draft, clonePlain(value)); storageUnit.value = Number(draft.maxDiskBytes || 0) >= 1024 ** 3 ? 'GB' : 'MB'; state.dirty = false }, { immediate: true, deep: true })
const segmentSizeMB = computed({ get: () => Number((Number(draft.segmentMaxBytes || 0) / 1024 ** 2).toFixed(2)), set: (value) => { draft.segmentMaxBytes = Math.round(Number(value || 0) * 1024 ** 2) } })
const storageAmount = computed({ get: () => Number((Number(draft.maxDiskBytes || 0) / (storageUnit.value === 'GB' ? 1024 ** 3 : 1024 ** 2)).toFixed(2)), set: (value) => { draft.maxDiskBytes = Math.round(Number(value || 0) * (storageUnit.value === 'GB' ? 1024 ** 3 : 1024 ** 2)) } })
function change() { state.dirty = true }
async function save() { try { await props.invoke('SaveAppSettings', clonePlain(draft)); notification.success({ message: '设置已保存', description: '日志位置将在重启程序后生效', placement: 'bottomRight', duration: 3 }); state.dirty = false; emit('saved') } catch (_) {} }
async function open() { try { await props.invoke('OpenLogFolder', draft.defaultLogDirectory) } catch (_) {} }
async function openLogs() { try { await props.invoke('OpenAppLogDirectory') } catch (_) {} }
</script>
<template>
  <main class="page-view">
    <div class="page-heading"><h1>全局设置</h1><Button type="primary" :disabled="!state.dirty" @click="save"><template #icon><SaveOutlined /></template>保存设置</Button></div>
    <div class="settings-grid">
      <section class="settings-card"><h2>日志与显示</h2><Form layout="vertical">
        <FormItem label="默认日志位置"><Input v-model:value="draft.defaultLogDirectory" @input="change"><template #addonAfter><Button type="text" aria-label="打开日志目录" @click="open"><FolderOpenOutlined /></Button></template></Input></FormItem>
        <div class="settings-fields"><FormItem label="分段时间（秒）"><InputNumber v-model:value="draft.segmentMaxAgeSeconds" :min="10" @change="change" /></FormItem><FormItem label="分段大小"><InputNumber v-model:value="segmentSizeMB" :min="1" @change="change"><template #addonAfter>MB</template></InputNumber></FormItem><FormItem label="无日志告警（秒）"><InputNumber v-model:value="draft.noLogTimeoutSeconds" :min="1" @change="change" /></FormItem><FormItem label="窗口最大行数"><InputNumber v-model:value="draft.maxLogLines" :min="100" @change="change" /></FormItem></div>
      </Form><div class="setting-row"><Tooltip title="关闭后允许横向滚动"><strong>默认自动换行</strong></Tooltip><Switch class="capsule-switch" :checked="Boolean(draft.autoWrap)" @change="(value) => { draft.autoWrap = value; change() }" /></div></section>
      <section class="settings-card"><h2>存储保护</h2><Form layout="vertical"><div class="settings-fields"><FormItem label="最大本地占用"><div class="unit-input"><InputNumber v-model:value="storageAmount" :min="1" @change="change" /><Select v-model:value="storageUnit" :options="[{ value: 'MB', label: 'MB' }, { value: 'GB', label: 'GB' }]" /></div></FormItem><FormItem label="预警阈值（%）"><InputNumber v-model:value="draft.storageWarningPercent" :min="1" :max="99" @change="change" /></FormItem></div></Form><div class="setting-row"><Tooltip title="默认关闭；未上传和待核对文件永不自动删除"><strong>自动删除已上传文件</strong></Tooltip><Switch class="capsule-switch" :checked="Boolean(draft.autoDeleteUploaded)" @change="(value) => { draft.autoDeleteUploaded = value; change() }" /></div></section>
      <section class="settings-card"><h2>云端上传</h2><Form layout="vertical"><Tooltip title="保存后重启程序生效；云端迁移时无需重装，改这里即可"><FormItem label="后端地址"><Input v-model:value="draft.backendUrl" placeholder="http://host:port/api" @input="change" /></FormItem></Tooltip><div class="settings-fields"><FormItem label="上传间隔（秒）"><InputNumber v-model:value="draft.uploadIntervalSeconds" disabled /></FormItem><FormItem label="并发数"><InputNumber v-model:value="draft.uploadConcurrency" disabled /></FormItem></div></Form><div class="setting-row"><Tooltip title="按服务端契约发送"><strong>GZIP 压缩</strong></Tooltip><Switch class="capsule-switch" :checked="Boolean(draft.uploadGzip)" @change="(value) => { draft.uploadGzip = value; change() }" /></div><div class="setting-row"><Tooltip title="上传失败原因与重试轨迹记录在本地日志文件"><strong>诊断日志</strong></Tooltip><Button size="small" @click="openLogs"><template #icon><FolderOpenOutlined /></template>打开日志目录</Button></div></section>
      <section class="settings-card about-card"><h2>程序信息</h2><Descriptions :column="1" size="small" bordered><DescriptionsItem label="程序名称">{{ draft.programName }}</DescriptionsItem><DescriptionsItem label="程序版本">{{ draft.programVersion }}</DescriptionsItem><DescriptionsItem label="构建版本">{{ draft.buildVersion }}</DescriptionsItem><DescriptionsItem label="发布日期">{{ draft.updateDate }}</DescriptionsItem></Descriptions><div class="company-merged">上海七十迈数字科技有限公司</div><div class="community"><TeamOutlined /><div class="community-content"><Tooltip :title="draft.communityText || '使用说明、问题反馈、获取最新版本请扫码加入飞书交流群。'"><strong>{{ draft.communityTitle || '飞书交流群' }}</strong></Tooltip><AntImage class="community-qr" src="/community-qr.png" :width="148" alt="飞书交流群二维码" :preview="{ mask: '查看大图' }" /><Button v-if="draft.communityUrl" type="link" :href="draft.communityUrl" target="_blank">打开加入链接</Button></div></div></section>
    </div>
  </main>
</template>
