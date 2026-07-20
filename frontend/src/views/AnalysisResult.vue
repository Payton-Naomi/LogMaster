<template>
  <div class="page" v-loading="loading">
    <header><div class="title"><el-button text circle :icon="ArrowLeft" @click="router.push(`/task/${taskId}`)" /><div><h1>解析结果</h1><p>{{ task.original_name || taskId }}</p></div></div><el-button :icon="Download" :disabled="!results.length" @click="exportResults">导出命中日志</el-button></header>
    <section v-if="task.task_id" class="summary"><div><span>日志总行数</span><strong>{{ task.total_lines.toLocaleString() }}</strong></div><div><span>错误</span><strong class="error">{{ task.error_count }}</strong></div><div><span>警告</span><strong class="warning">{{ task.warning_count }}</strong></div><div><span>Agent 诊断</span><strong>{{ agentFindings.length }}</strong></div></section>
    <section class="panel">
      <div class="filters"><el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索关键字、文件或日志内容" /><el-select v-model="level" clearable placeholder="全部级别"><el-option label="错误" value="error" /><el-option label="警告" value="warning" /></el-select><span>共 {{ filtered.length }} 条</span></div>
      <el-table :data="paged"><el-table-column prop="level" label="级别" width="90"><template #default="scope"><el-tag :type="scope.row.level==='error'?'danger':'warning'" effect="plain">{{ scope.row.level==='error'?'错误':'警告' }}</el-tag></template></el-table-column><el-table-column prop="matched_text" label="关键字" width="120" /><el-table-column prop="file_path" label="文件" min-width="190" /><el-table-column prop="line_number" label="行号" width="80" /><el-table-column prop="content" label="日志内容" min-width="360" show-overflow-tooltip /><template #empty><el-empty description="数据库中暂无解析结果" /></template></el-table>
      <footer><span>显示真实解析记录</span><el-pagination v-model:current-page="page" :page-size="pageSize" :total="filtered.length" layout="prev, pager, next" /></footer>
    </section>
    <section v-if="agentFindings.length" class="panel agent"><h2>Agent 诊断</h2><div v-for="(finding,index) in agentFindings" :key="index" class="finding"><div><strong>{{ finding.category || '未分类' }}</strong><el-tag effect="plain">{{ finding.severity }}</el-tag></div><p>{{ finding.root_cause }}</p><span>{{ finding.suggestion }}</span></div></section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Download, Search } from '@element-plus/icons-vue'
import { getAgentResults, getTaskDetail, getTaskResults } from '@/api/task'

const route=useRoute();const router=useRouter();const taskId=route.params.taskId;const loading=ref(false);const task=ref({});const results=ref([]);const agentResults=ref([]);const search=ref('');const level=ref('');const page=ref(1);const pageSize=20
const agentFindings=computed(()=>agentResults.value.flatMap(item=>item.findings||[]))
const filtered=computed(()=>{const text=search.value.trim().toLowerCase();return results.value.filter(item=>(!level.value||item.level===level.value)&&(!text||`${item.matched_text}${item.file_path}${item.content}`.toLowerCase().includes(text)))})
const paged=computed(()=>filtered.value.slice((page.value-1)*pageSize,page.value*pageSize))
async function load(){loading.value=true;try{const [detail,parsed,agents]=await Promise.all([getTaskDetail(taskId),getTaskResults(taskId,{page:1,page_size:200}),getAgentResults(taskId)]);task.value=detail.task;results.value=parsed;agentResults.value=agents}finally{loading.value=false}}
function exportResults(){const content=results.value.map(item=>`${item.file_path}:${item.line_number}\t${item.level}\t${item.matched_text}\t${item.content}`).join('\n');const url=URL.createObjectURL(new Blob([content],{type:'text/plain;charset=utf-8'}));const link=document.createElement('a');link.href=url;link.download=`${taskId}-results.txt`;link.click();URL.revokeObjectURL(url)}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header,.title,.filters,.finding>div{display:flex;align-items:center}.page>header{justify-content:space-between;margin-bottom:18px}.title{gap:8px}.title h1{margin:0;font-size:21px}.title p{margin:4px 0 0;color:#8a94a3;font:11px Consolas,monospace}.summary{display:grid;grid-template-columns:repeat(4,1fr);margin-bottom:16px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.summary div{display:flex;align-items:center;flex-direction:column;gap:6px;padding:18px;border-right:1px solid #edf0f3}.summary div:last-child{border:0}.summary span{color:#8a94a3;font-size:11px}.summary strong{font-size:22px}.summary .error{color:#d95858}.summary .warning{color:#c9861b}.panel{padding:17px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.filters{gap:10px;margin-bottom:14px}.filters .el-input{width:min(430px,50%)}.filters .el-select{width:130px}.filters>span{margin-left:auto;color:#8a94a3;font-size:11px}footer{display:flex;align-items:center;justify-content:space-between;padding-top:15px;color:#8a94a3;font-size:11px}.agent{margin-top:16px}.agent h2{margin:0 0 12px;font-size:15px}.finding{padding:12px 0;border-bottom:1px solid #edf0f3}.finding>div{gap:8px}.finding p{margin:7px 0;color:#3d4859}.finding>span{color:#687487;font-size:12px}@media(max-width:700px){.summary{grid-template-columns:repeat(2,1fr)}.filters{flex-wrap:wrap}.filters .el-input,.filters .el-select{width:100%}.filters>span{margin-left:0}}
</style>
