<template>
  <div class="page">
    <header><div><h1>日志上传与解析</h1><p>上传后由后端保存、解压并异步解析</p></div><span>{{ logicalCount }} 个日志文件 / {{ formatSize(totalSize) }}</span></header>
    <div class="workspace">
      <section class="panel">
        <div class="panel-title"><h2>选择日志</h2><el-button v-if="files.length" text :icon="Delete" :disabled="submitting" @click="files=[]">清空</el-button></div>
        <input ref="input" type="file" hidden multiple @change="selectFiles">
        <div class="drop-zone" @click="input?.click()" @dragover.prevent @drop.prevent="dropFiles">
          <el-icon><UploadFilled /></el-icon><strong>拖入日志或压缩包</strong><span>支持 LOG、TXT、OUT、CSV、ZIP、GZ、TGZ、TAR.GZ</span>
          <el-button type="primary" :icon="DocumentAdd" @click.stop="input?.click()">选择文件</el-button>
        </div>
        <div class="file-list">
          <el-empty v-if="!displayEntries.length" description="尚未选择文件" :image-size="70" />
          <div v-for="entry in displayEntries" :key="entry.key" class="file-row">
            <span class="ext">{{ extension(entry.path) }}</span><div><strong>{{ entry.path }}</strong><small>{{ entry.sizeBytes ? formatSize(entry.sizeBytes) : '大小将在解压后确认' }}<template v-if="entry.encrypted"> · 已加密</template></small></div>
            <el-button text circle :icon="Close" :disabled="submitting" title="移除来源文件" @click="removeFile(entry.sourceIndex)" />
          </div>
        </div>
      </section>

      <section class="panel form-panel">
        <h2>上传信息</h2>
        <el-form label-position="top">
          <el-form-item label="项目名称"><el-input v-model="projectName" maxlength="128" placeholder="例如 DR2860" /></el-form-item>
          <el-form-item label="版本"><el-input v-model="version" maxlength="64" placeholder="例如 V1.2.0（可选）" /></el-form-item>
        </el-form>
        <el-button type="primary" size="large" :icon="VideoPlay" :loading="submitting" :disabled="!files.length || !projectName.trim()" @click="submit">
          {{ submitting ? '正在处理' : '上传并解析' }}
        </el-button>

        <div v-if="task" class="task-state">
          <div><span>任务状态</span><el-tag :type="statusMeta.type" effect="plain">{{ statusMeta.label }}</el-tag></div>
          <p>{{ task.task_id }}</p>
          <el-progress :percentage="progress" :status="task.status === 'failed' ? 'exception' : task.status === 'completed' ? 'success' : ''" />
          <dl><div><dt>日志行数</dt><dd>{{ task.total_lines.toLocaleString() }}</dd></div><div><dt>错误</dt><dd>{{ task.error_count }}</dd></div><div><dt>警告</dt><dd>{{ task.warning_count }}</dd></div></dl>
          <el-alert v-if="task.error_message" :title="task.error_message" type="error" :closable="false" />
          <el-button v-if="task.status === 'completed'" type="primary" link @click="router.push(`/analysis/${task.task_id}`)">查看真实解析结果</el-button>
        </div>
        <el-empty v-else description="任务状态将在上传后显示" :image-size="80" />
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Close, Delete, DocumentAdd, UploadFilled, VideoPlay } from '@element-plus/icons-vue'
import { inspectLog, uploadLogs } from '@/api/log'
import { getTaskDetail } from '@/api/task'

const router = useRouter()
const input = ref(null)
const files = ref([])
const projectName = ref('')
const version = ref('')
const submitting = ref(false)
const task = ref(null)
let pollTimer = null
const accepted = /\.(log|txt|out|csv|zip|gz|tgz)$/i
const totalSize = computed(() => files.value.reduce((sum,item)=>sum+item.raw.size,0))
const logicalCount = computed(() => files.value.reduce((sum,item)=>sum+item.entries.length,0))
const displayEntries = computed(() => files.value.flatMap((item,sourceIndex)=>item.entries.map((entry,index)=>({...entry,sourceIndex,key:`${sourceIndex}:${index}:${entry.path}`}))))
const progress = computed(() => ({ uploading: 10, queued: 25, parsing: 65, completed: 100, failed: 100 })[task.value?.status] || 0)
const statusMeta = computed(() => ({ uploading:{label:'上传中',type:'info'},queued:{label:'排队中',type:'info'},parsing:{label:'解析中',type:'primary'},completed:{label:'已完成',type:'success'},failed:{label:'失败',type:'danger'} })[task.value?.status] || {label:'等待',type:'info'})

async function addFiles(items) {
  const existing = new Set(files.value.map((item)=>`${item.raw.name}:${item.raw.size}:${item.raw.lastModified}`))
  let rejected = 0
  for (const file of items) {
    const key = `${file.name}:${file.size}:${file.lastModified}`
    const extensionless = !file.name.includes('.')
    if ((!accepted.test(file.name) && !extensionless) || existing.has(key)) { rejected++; continue }
    try {
      const isArchive = /\.(zip|gz|tgz)$/i.test(file.name)
      const inspection = isArchive ? await inspectLog(file) : { archive:false, entries:[{path:file.name,size_bytes:file.size,encrypted:false}] }
      existing.add(key)
      files.value.push({ raw:file, archive:inspection.archive, entries:inspection.entries.map(entry=>({path:entry.path,sizeBytes:entry.size_bytes,encrypted:entry.encrypted})) })
    } catch { rejected++ }
  }
  if (rejected) ElMessage.warning(`${rejected} 个文件格式不支持或已重复`)
}
async function selectFiles(event){ await addFiles(Array.from(event.target.files)); event.target.value='' }
async function dropFiles(event){ await addFiles(Array.from(event.dataTransfer.files)) }
function removeFile(index){ files.value.splice(index,1) }
function extension(name){ return name.includes('.') ? name.split('.').pop()?.toUpperCase() : 'LOG' }
function formatSize(bytes){ if(!bytes)return'0 B';const units=['B','KB','MB','GB'];const i=Math.min(Math.floor(Math.log(bytes)/Math.log(1024)),3);return`${(bytes/1024**i).toFixed(i?1:0)} ${units[i]}` }

async function submit(){
  submitting.value=true; task.value=null
  try {
    const created=await uploadLogs(files.value.map(item=>item.raw),projectName.value.trim(),version.value.trim())
    task.value={task_id:created.task_id,status:created.status,total_lines:0,error_count:0,warning_count:0,error_message:''}
    ElMessage.success('日志已上传，后端开始解析')
    await poll(created.task_id)
  } finally { submitting.value=false }
}
async function poll(taskId){
  const data=await getTaskDetail(taskId); task.value=data.task
  if(task.value.status==='completed'||task.value.status==='failed') return
  pollTimer=window.setTimeout(()=>poll(taskId).catch(()=>{}),1000)
}
onBeforeUnmount(()=>window.clearTimeout(pollTimer))
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page h1{margin:0;font-size:22px}.page header p{margin:5px 0 0;color:#7a8493;font-size:13px}.page header>span{color:#667085;font-size:12px}.workspace{display:grid;grid-template-columns:minmax(0,1.25fr) minmax(330px,.75fr);gap:16px}.panel{padding:18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.panel-title{display:flex;align-items:center;justify-content:space-between}.panel h2{margin:0 0 16px;font-size:16px}.drop-zone{display:flex;min-height:210px;align-items:center;justify-content:center;flex-direction:column;gap:10px;border:1px dashed #b8c4d3;border-radius:6px;background:#f8fafc;cursor:pointer}.drop-zone>.el-icon{color:#3478dc;font-size:38px}.drop-zone span{color:#8a94a3;font-size:12px}.file-list{max-height:360px;margin-top:14px;overflow:auto}.file-row{display:grid;grid-template-columns:42px minmax(0,1fr) 34px;align-items:center;gap:10px;padding:10px 4px;border-bottom:1px solid #edf0f3}.ext{display:grid;width:38px;height:32px;place-items:center;border-radius:4px;background:#edf4ff;color:#3478dc;font-size:10px;font-weight:700}.file-row div{display:flex;min-width:0;flex-direction:column;gap:4px}.file-row strong{overflow:hidden;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.file-row small{color:#8a94a3}.form-panel>.el-button{width:100%}.task-state{margin-top:20px;padding-top:18px;border-top:1px solid #edf0f3}.task-state>div:first-child{display:flex;align-items:center;justify-content:space-between}.task-state p{overflow:hidden;color:#8a94a3;font:11px Consolas,monospace;text-overflow:ellipsis}.task-state dl{display:grid;grid-template-columns:repeat(3,1fr);margin:16px 0}.task-state dl div{display:flex;align-items:center;flex-direction:column;gap:4px}.task-state dt{color:#8a94a3;font-size:11px}.task-state dd{margin:0;font-size:18px;font-weight:600}@media(max-width:900px){.workspace{grid-template-columns:1fr}}@media(max-width:600px){.page>header{align-items:flex-start;flex-direction:column;gap:10px}.panel{padding:13px}}
</style>
