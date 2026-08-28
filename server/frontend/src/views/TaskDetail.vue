<template>
  <div class="page task-detail-page" v-loading="loading">
    <header><div class="title"><el-button text circle :icon="ArrowLeft" @click="router.push('/tasks')" /><div><h1>{{ task.original_name || task.project_name || '任务详情' }}</h1><p>{{ task.task_id }}</p></div></div><div class="header-actions"><el-button v-if="task.status === 'failed'" type="warning" @click="runAction('retry')">重新解析</el-button><el-button v-if="['queued','parsing','running'].includes(task.status)" @click="runAction('pause')">暂停</el-button><el-button v-if="task.status === 'paused'" @click="runAction('resume')">恢复</el-button><el-button v-if="['queued','parsing','running','paused'].includes(task.status)" type="danger" plain @click="runAction('cancel')">取消任务</el-button><el-button type="primary" :icon="DataAnalysis" :disabled="!task.task_id" @click="router.push(`/analysis/${task.task_id}`)">解析结果</el-button></div></header>
    <el-alert v-if="loadError" class="page-error" :title="loadError" type="error" show-icon :closable="false" />
    <div v-if="task.task_id" class="overview">
      <section class="panel meta"><div class="status"><div><el-tag :type="statusMeta.type" effect="plain">{{ statusMeta.label }}</el-tag><el-tag class="ai-tag" :type="aiStatusMeta.type" effect="plain">AI：{{ aiStatusMeta.label }}</el-tag></div><span>{{ formatDate(task.updated_at) }}</span></div><el-progress v-if="!['completed','failed','cancelled'].includes(task.status)" :percentage="progress" :status="statusMeta.type === 'danger' ? 'exception' : ''" /><dl><div><dt>项目</dt><dd>{{ task.project_name }}</dd></div><div><dt>版本</dt><dd>{{ task.version || '-' }}</dd></div><div><dt>文件数量</dt><dd>{{ task.file_count }}</dd></div><div><dt>原始大小</dt><dd>{{ formatSize(task.original_size) }}</dd></div><div><dt>日志行数</dt><dd>{{ Number(task.total_lines || 0).toLocaleString() }}</dd></div><div><dt>创建时间</dt><dd>{{ formatDate(task.created_at) }}</dd></div></dl></section>
      <section class="panel counts"><h2>解析统计</h2><div><span><strong class="error">{{ task.error_count || 0 }}</strong><small>错误</small></span><span><strong class="warning">{{ task.warning_count || 0 }}</strong><small>警告</small></span><span><strong>{{ results.length }}</strong><small>结果记录</small></span></div><el-alert v-if="task.error_message" :title="task.error_message" type="error" :closable="false" /></section>
    </div>
    <div v-if="task.task_id" class="content">
      <section class="panel"><div class="panel-heading"><div><h2>日志文件</h2><p>{{ files.length > 1 ? '可点击左侧任意文件，单独查看该文件的解析结果' : '数据库记录的实际解压文件' }}</p></div><div class="download-actions"><el-button link @click="downloadExport('original')">下载原始包</el-button><el-button link @click="downloadExport('batch')">下载解析包</el-button></div></div><el-table :data="files" @row-click="openFileResults" :row-class-name="() => files.length > 1 ? 'file-row-clickable' : ''"><el-table-column prop="relative_path" label="文件路径" min-width="260" /><el-table-column label="大小" width="110"><template #default="scope">{{ formatSize(scope.row.size_bytes) }}</template></el-table-column><el-table-column label="行数" width="110"><template #default="scope">{{ Number(scope.row.line_count || 0).toLocaleString() }}</template></el-table-column><el-table-column v-if="files.length > 1" label="操作" width="145"><template #default="scope"><el-button link type="primary" @click.stop="openFileResults(scope.row)">查看解析结果</el-button></template></el-table-column><template #empty><el-empty description="暂无文件记录" :image-size="70" /></template></el-table></section>
      <section class="panel"><div class="panel-heading"><div><h2>命中摘要</h2><p>当前接口返回的真实解析结果</p></div><div class="download-actions"><el-button link @click="downloadExport('csv')">CSV</el-button><el-button link @click="downloadExport('report')">报告</el-button><span>{{ results.length }} 条</span></div></div><el-table :data="resultSummary"><el-table-column prop="matched_text" label="关键字" /><el-table-column prop="level" label="级别" width="90" /><el-table-column prop="count" label="次数" width="80" /><template #empty><el-empty description="暂无命中结果" :image-size="70" /></template></el-table></section>
    </div>
    <el-empty v-if="!loading && !task.task_id" description="任务不存在" />
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, DataAnalysis } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { cancelTask, exportTask, getTaskDetail, getTaskResults, pauseTask, retryTask, resumeTask } from '@/api/task'
import { downloadLog } from '@/api/log'

const route=useRoute();const router=useRouter();const loading=ref(false);const loadError=ref('');const task=ref({});const files=ref([]);const results=ref([]);let pollTimer=null
const statusMeta=computed(()=>({uploading:{label:'上传中',type:'info'},queued:{label:'排队中',type:'info'},parsing:{label:'解析中',type:'primary'},running:{label:'解析中',type:'primary'},paused:{label:'已暂停',type:'warning'},cancelled:{label:'已取消',type:'info'},completed:{label:'已完成',type:'success'},failed:{label:'失败',type:'danger'}})[task.value.status]||{label:'未知',type:'info'})
const aiStatusMeta=computed(()=>({disabled:{label:'未启用',type:'info'},queued:{label:'排队中',type:'warning'},running:{label:'分析中',type:'primary'},completed:{label:'已完成',type:'success'},partial_failed:{label:'部分失败',type:'warning'},failed:{label:'失败',type:'danger'},cancelled:{label:'已取消',type:'info'}})[task.value.ai_status]||{label:'未知',type:'info'})
const progress=computed(()=>Math.max(0,Math.min(100,Number(task.value.progress)||(['completed','cancelled','failed'].includes(task.value.status)?100:0))))
const resultSummary=computed(()=>{const map=new Map();for(const item of results.value){const key=`${item.level}:${item.matched_text}`;const current=map.get(key)||{level:item.level,matched_text:item.matched_text,count:0};current.count++;map.set(key,current)}return[...map.values()]})
const formatSize=(bytes)=>{if(!bytes)return'0 B';const units=['B','KB','MB','GB'];const i=Math.min(Math.floor(Math.log(bytes)/Math.log(1024)),3);return`${(bytes/1024**i).toFixed(i?1:0)} ${units[i]}`}
const formatDate=(value)=>value?new Date(value).toLocaleString('zh-CN',{hour12:false}):'-'
const errorMessage=(error,fallback)=>error?.response?.data?.message||error?.message||fallback
async function load({silent=false}={}){if(!silent)loading.value=true;if(!silent)loadError.value='';try{const [detail,parsed]=await Promise.all([getTaskDetail(route.params.taskId),getTaskResults(route.params.taskId,{page:1,page_size:200})]);task.value=detail.task;files.value=detail.files||[];results.value=parsed||[]}catch(error){if(!silent){loadError.value=errorMessage(error,'加载任务详情失败');task.value={};files.value=[];results.value=[]}}finally{if(!silent)loading.value=false}}
async function runAction(action){const labels={retry:'重新解析',pause:'暂停任务',resume:'恢复任务',cancel:'取消任务'};try{if(['pause','cancel'].includes(action))await ElMessageBox.confirm(`确认${labels[action]}？`,labels[action],{type:'warning',confirmButtonText:'确认',cancelButtonText:'取消'});await({retry:retryTask,pause:pauseTask,resume:resumeTask,cancel:cancelTask}[action])(route.params.taskId);ElMessage.success(`${labels[action]}请求已提交`);await load()}catch(error){if(error!=='cancel'&&error!=='close')ElMessage.error(errorMessage(error,`${labels[action]}失败，请稍后重试`))}}
function openFileResults(file){if(files.value.length <= 1 || !file?.relative_path) return; router.push({path:`/analysis/${route.params.taskId}`,query:{file_id:String(file.id || ''),file_path:file.relative_path}})}
async function downloadExport(format){try{const response=['original','batch'].includes(format)?await downloadLog(task.value.id,{type:format}):await exportTask(route.params.taskId,format);const blob=new Blob([response]);const link=document.createElement('a');link.href=URL.createObjectURL(blob);link.download=`logmaster-${route.params.taskId}.${['original','batch'].includes(format)?'zip':format==='report'?'md':format}`;link.click();URL.revokeObjectURL(link.href)}catch(error){ElMessage.error(errorMessage(error,'下载失败，请稍后重试'))}}
onMounted(()=>{load();pollTimer=window.setInterval(()=>{if(['queued','parsing','running'].includes(task.value.status)||['queued','running'].includes(task.value.ai_status))load({silent:true})},5000)})
onBeforeUnmount(()=>window.clearInterval(pollTimer))
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header,.title,.status,.panel-heading{display:flex;align-items:center}.page>header{justify-content:space-between;margin-bottom:18px}.page-error{margin-bottom:16px}.title{gap:8px}.title h1{margin:0;font-size:21px}.title p{margin:4px 0 0;color:#8a94a3;font:11px Consolas,monospace}.overview{display:grid;grid-template-columns:1.4fr .6fr;gap:16px;margin-bottom:16px}.panel{padding:18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.status{justify-content:space-between;padding-bottom:14px;border-bottom:1px solid #edf0f3;color:#8a94a3;font-size:11px}.meta dl{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;margin:16px 0 0}.meta dl div{display:flex;min-width:0;flex-direction:column;gap:5px}.meta dt{color:#8a94a3;font-size:11px}.meta dd{overflow:hidden;margin:0;font-size:13px;text-overflow:ellipsis;white-space:nowrap}.counts h2,.panel-heading h2{margin:0;font-size:15px}.counts>div{display:grid;grid-template-columns:repeat(3,1fr);margin:22px 0}.counts span{display:flex;align-items:center;flex-direction:column;gap:5px}.counts strong{font-size:23px}.counts small{color:#8a94a3}.counts .error{color:#d95858}.counts .warning{color:#c9861b}.content{display:grid;grid-template-columns:1fr 1fr;gap:16px}.panel-heading{justify-content:space-between;margin-bottom:10px}.panel-heading p{margin:4px 0 0;color:#8a94a3;font-size:11px}.panel-heading>span{color:#8a94a3;font-size:12px}@media(max-width:900px){.overview,.content{grid-template-columns:1fr}.meta dl{grid-template-columns:repeat(2,1fr)}}
</style>

<style>
html[data-log-theme="light"] .task-detail-page {
  background: radial-gradient(circle at 50% 8%, rgba(255,255,255,.64), transparent 34%), linear-gradient(145deg, rgba(215,231,235,.82), rgba(239,246,247,.72) 48%, rgba(198,217,223,.84)) !important;
  color: #17303b !important;
}
html[data-log-theme="light"] .task-detail-page > header,
html[data-log-theme="light"] .task-detail-page .panel { background: linear-gradient(145deg, rgba(255,255,255,.5), rgba(224,240,244,.28)) !important; border-color: rgba(67,98,112,.24) !important; box-shadow: inset 0 1px rgba(255,255,255,.82), 0 18px 48px rgba(35,67,83,.16) !important; }
html[data-log-theme="light"] .task-detail-page .title h1,
html[data-log-theme="light"] .task-detail-page .counts h2,
html[data-log-theme="light"] .task-detail-page .panel-heading h2,
html[data-log-theme="light"] .task-detail-page .meta dd,
html[data-log-theme="light"] .task-detail-page .counts strong { color: #17303b !important; }
html[data-log-theme="light"] .task-detail-page .title p,
html[data-log-theme="light"] .task-detail-page .status,
html[data-log-theme="light"] .task-detail-page .meta dt,
html[data-log-theme="light"] .task-detail-page .counts small,
html[data-log-theme="light"] .task-detail-page .panel-heading p,
html[data-log-theme="light"] .task-detail-page .panel-heading > span { color: #55727d !important; }
html[data-log-theme="light"] .task-detail-page .meta dl > div,
html[data-log-theme="light"] .task-detail-page .counts > div > span { background: rgba(244,250,251,.82) !important; border-color: rgba(67,98,112,.16) !important; }
html[data-log-theme="light"] .task-detail-page .panel .el-table { --el-table-text-color: #263d47; --el-table-header-text-color: #55727d; --el-table-header-bg-color: rgba(67,98,112,.08); --el-table-border-color: rgba(67,98,112,.16); }
html[data-log-theme="light"] .task-detail-page .panel th.el-table__cell { background: rgba(67,98,112,.08) !important; color: #55727d !important; }
html[data-log-theme="light"] .task-detail-page .panel td.el-table__cell { color: #263d47 !important; border-bottom-color: rgba(67,98,112,.14) !important; }
</style>
<style scoped>
.task-detail-page { min-width: 0; overflow-x: hidden; }
.task-detail-page > header { min-width: 0; gap: 16px; }
.task-detail-page .title { min-width: 0; flex: 1 1 auto; }
.task-detail-page .title > div { min-width: 0; }
.task-detail-page .title h1 { overflow: hidden; min-width: 0; text-overflow: ellipsis; white-space: nowrap; }
.task-detail-page .header-actions { display: flex; flex: 0 1 auto; flex-wrap: wrap; justify-content: flex-end; gap: 8px; min-width: 0; }
.task-detail-page .overview, .task-detail-page .content { min-width: 0; }
.task-detail-page .panel { min-width: 0; overflow: hidden; }
.task-detail-page :deep(.el-table) { max-width: 100%; }
@media (max-width: 760px) {
  .task-detail-page > header { align-items: stretch; }
  .task-detail-page .title h1 { white-space: normal; overflow-wrap: anywhere; }
  .task-detail-page .header-actions { justify-content: stretch; }
  .task-detail-page .header-actions .el-button { flex: 1 1 auto; min-width: 0; }
}
</style>
<style scoped>
.file-row-clickable{cursor:pointer}
.file-row-clickable:hover>td{background:rgba(6,182,212,.1)!important}
</style>
<style scoped>.download-actions .el-button:nth-child(2){display:none!important}.download-actions .el-button:first-child{padding:10px 16px;font-size:14px}</style>

<style scoped>
:global(.page){font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif!important;-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility;background:radial-gradient(circle at 50% 7%,rgba(255,255,255,.1),transparent 32%),radial-gradient(circle at 50% 85%,rgba(56,189,248,.07),transparent 52%),linear-gradient(145deg,#14171b,#1b1f25 48%,#111418)!important;color:#f5f7fa!important}.page>header,.panel{background:rgba(255,255,255,.085)!important;border:1px solid rgba(255,255,255,.16)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.12),0 18px 52px rgba(0,0,0,.28)!important;border-radius:14px!important}.page>header{padding:18px 20px;margin-bottom:14px!important;animation:detail-rise .55s cubic-bezier(.4,0,.2,1) both}.title h1,.counts h2,.panel-heading h2,.meta dd{color:#f5f7fa!important}.title p,.status,.meta dt,.counts small,.panel-heading p,.panel-heading>span{color:#b7bec8!important}.title p,.status span,.meta dd,.panel :deep(td.el-table__cell){font-variant-numeric:tabular-nums}.title :deep(.el-button){width:36px;height:36px;background:rgba(255,255,255,.07);border:1px solid rgba(255,255,255,.14);color:#c5ced8}.page>header>.el-button{border:0;border-radius:9px;background:linear-gradient(135deg,#0891b2,#06b6d4);box-shadow:0 9px 24px rgba(6,182,212,.22);transition:all .24s cubic-bezier(.4,0,.2,1)}.page>header>.el-button:active,.title :deep(.el-button:active){transform:scale(.94)}.overview,.content{gap:12px!important}.panel{padding:18px!important;animation:detail-rise .5s cubic-bezier(.4,0,.2,1) both}.status{border-bottom-color:rgba(255,255,255,.12)!important}.status :deep(.el-tag){background:rgba(52,211,153,.1);border-color:rgba(52,211,153,.32);color:#6ee7b7}.meta dl{gap:12px!important}.meta dl div{padding:11px 12px;border:1px solid rgba(255,255,255,.09);border-radius:9px;background:rgba(0,0,0,.14)}.counts>div{gap:8px}.counts span{padding:14px 8px;border:1px solid rgba(255,255,255,.09);border-radius:9px;background:rgba(0,0,0,.14)}.counts strong{color:#f5f7fa}.counts .error{color:#fb7185!important}.counts .warning{color:#fbbf24!important}.panel :deep(.el-table){--el-table-bg-color:transparent;--el-table-tr-bg-color:transparent;--el-table-header-bg-color:rgba(255,255,255,.07);--el-table-row-hover-bg-color:rgba(6,182,212,.1);--el-table-border-color:rgba(255,255,255,.1);--el-table-text-color:#eef2f6;--el-table-header-text-color:#b7bec8;background:transparent!important}.panel :deep(th.el-table__cell){background:rgba(255,255,255,.07)!important;color:#c4ccd6!important;border-bottom-color:rgba(255,255,255,.13)!important}.panel :deep(td.el-table__cell){background:transparent;border-bottom-color:rgba(255,255,255,.1)!important}.panel :deep(.el-table__row:hover>td.el-table__cell){background:rgba(6,182,212,.1)!important}.panel :deep(.el-table__row:hover td:first-child){box-shadow:inset 2px 0 #38bdf8}.panel :deep(.el-empty__description){color:#b7bec8}.page-error{border:1px solid rgba(244,63,94,.25);background:rgba(244,63,94,.1)}@keyframes detail-rise{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:none}}@media(prefers-reduced-motion:reduce){*,*:before,*:after{animation-duration:.01ms!important;transition-duration:.01ms!important}}@media(max-width:650px){.page>header{align-items:flex-start;flex-direction:column;gap:14px}.page>header>.el-button{width:100%}.meta dl{grid-template-columns:1fr 1fr}}
</style>
