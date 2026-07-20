<template>
  <div class="page">
    <header><div><h1>实时日志</h1><p>展示当前机器真实串口状态，不生成模拟日志</p></div><el-button :icon="Refresh" :loading="loading" @click="load">刷新设备</el-button></header>
    <section class="toolbar panel"><el-form inline><el-form-item label="项目"><el-select v-model="project" placeholder="选择已有项目"><el-option v-for="item in projects" :key="item" :label="item" :value="item" /></el-select></el-form-item><el-form-item label="串口"><el-select v-model="port" placeholder="选择真实串口"><el-option v-for="item in ports" :key="item" :label="item" :value="item" /></el-select></el-form-item><el-form-item label="波特率"><el-select v-model="baudRate"><el-option v-for="item in baudRates" :key="item" :label="item" :value="item" /></el-select></el-form-item></el-form><el-alert title="当前后端尚未实现串口采集会话接口，因此不会生成或播放示例日志。" type="info" :closable="false" /></section>
    <section class="console panel"><div class="console-heading"><div><h2>日志输出</h2><p>连接真实采集服务后在此显示</p></div><span>{{ logs.length }} 行</span></div><div class="log-window"><div v-for="(line,index) in logs" :key="index">{{ line }}</div><el-empty v-if="!logs.length" description="暂无真实串口日志" /></div></section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { getComPorts, getProjects } from '@/api/log'

const loading=ref(false);const projects=ref([]);const ports=ref([]);const project=ref('');const port=ref('');const baudRate=ref('115200');const baudRates=['9600','19200','38400','57600','115200','230400'];const logs=ref([])
async function load(){loading.value=true;try{const [projectData,portData]=await Promise.all([getProjects(),getComPorts()]);projects.value=projectData;ports.value=portData;if(!projects.value.includes(project.value))project.value=projects.value[0]||'';if(!ports.value.includes(port.value))port.value=ports.value[0]||''}finally{loading.value=false}}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page h1{margin:0;font-size:22px}.page header p,.console-heading p{margin:5px 0 0;color:#7a8493;font-size:12px}.panel{padding:17px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.toolbar{margin-bottom:16px}.toolbar .el-form{margin-bottom:4px}.toolbar .el-select{width:180px}.console-heading{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:12px}.console-heading h2{margin:0;font-size:15px}.console-heading>span{color:#8a94a3;font-size:11px}.log-window{min-height:420px;max-height:620px;overflow:auto;padding:14px;border-radius:4px;background:#141b29;color:#dce5f2;font:12px/1.7 Consolas,monospace}.log-window .el-empty{height:390px}.log-window :deep(.el-empty__description p){color:#7f8ca0}@media(max-width:650px){.page>header{align-items:flex-start;flex-direction:column;gap:10px}.toolbar .el-form{display:block}.toolbar .el-select{width:100%}}
</style>
