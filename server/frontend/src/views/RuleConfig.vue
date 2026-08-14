<template>
  <div class="page rule-config">
    <header><div><h1>解析规则</h1><p>查看公共解析规则，并按需调整个人启用状态</p></div></header>
    <div class="summary"><div><span>规则总数</span><strong>{{ rules.length }}</strong></div><div><span>已启用</span><strong>{{ enabledCount }}</strong></div><div><span>严重级别</span><strong class="danger">{{ criticalCount }}</strong></div><div><span>场景已引用</span><strong>{{ usedCount }}</strong></div></div>
    <section class="panel">
      <div class="filters"><el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索名称、关键字或说明" /><el-select v-model="category" clearable placeholder="全部分类"><el-option v-for="item in categories" :key="item.value" :label="item.label" :value="item.value" /></el-select><el-button :icon="Refresh" :loading="loading" title="刷新规则" @click="load" /></div>
      <div class="level-filter" aria-label="按级别筛选">
        <span>级别</span>
        <button v-for="item in levelOptions" :key="item.value" type="button" :class="{ active: level === item.value }" @click="level = item.value">
          <i v-if="item.value" :class="item.value"></i>{{ item.label }}<em>{{ levelCount(item.value) }}</em>
        </button>
      </div>
      <div v-if="selection.length" class="batch-bar"><span>已选 <strong>{{ selection.length }}</strong> 项</span><el-button type="success" plain :loading="batchLoading" @click="batchSetEnabled(true)">批量启用</el-button><el-button type="warning" plain :loading="batchLoading" @click="batchSetEnabled(false)">批量停用</el-button><el-button text @click="clearSelection">取消选择</el-button></div>
      <el-table ref="tableRef" v-loading="loading" :data="filtered" row-key="id" @selection-change="selection = $event">
        <el-table-column type="selection" width="46" reserve-selection />
        <el-table-column prop="name" label="规则名称" min-width="170" />
        <el-table-column prop="category" label="分类" width="110"><template #default="scope">{{ categoryLabel(scope.row.category) }}</template></el-table-column>
        <el-table-column prop="keyword" label="关键字" min-width="260" show-overflow-tooltip />
        <el-table-column prop="scope" label="适用范围" min-width="120" />
        <el-table-column prop="scenario_count" label="场景引用" width="100" align="center"><template #default="scope"><el-tag v-if="scope.row.scenario_count" type="primary" effect="plain">{{ scope.row.scenario_count }} 个</el-tag><span v-else class="muted">未引用</span></template></el-table-column>
        <el-table-column prop="source" label="来源" width="100"><template #default="scope"><el-tag type="info" effect="plain">{{ sourceLabel(scope.row.source) }}</el-tag></template></el-table-column>
        <el-table-column prop="level" label="级别" width="90"><template #default="scope"><el-tag :type="levelType(scope.row.level)" effect="plain">{{ levelLabel(scope.row.level) }}</el-tag></template></el-table-column>
        <el-table-column label="启用" width="80"><template #default="scope"><el-switch v-model="scope.row.enabled" @change="saveExisting(scope.row)" /></template></el-table-column>
        <el-table-column label="操作" width="140"><template #default="scope"><el-button link type="primary" @click="openEdit(scope.row)">{{ scope.row.editable ? '编辑' : '查看' }}</el-button><el-tooltip v-if="scope.row.editable" :disabled="!scope.row.scenario_count" content="请先从测试场景中移除此规则" placement="top"><span><el-button link type="danger" :disabled="Boolean(scope.row.scenario_count)" @click="remove(scope.row)">删除</el-button></span></el-tooltip></template></el-table-column>
        <template #empty><el-empty description="数据库中暂无解析规则" /></template>
      </el-table>
    </section>

    <el-dialog v-model="dialog" class="rule-dialog" :title="form.editable ? '编辑规则' : '规则详情'" width="560px">
      <el-form label-position="top" :disabled="Boolean(form.id && !form.editable)"><div class="form-grid"><el-form-item label="规则名称"><el-input v-model="form.name" /></el-form-item><el-form-item label="分类"><el-select v-model="form.category"><el-option v-for="item in categories" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></div><el-form-item label="关键字"><el-input v-model="form.keyword" type="textarea" :rows="3" /></el-form-item><div class="form-grid"><el-form-item label="适用范围"><el-input v-model="form.scope" /></el-form-item><el-form-item label="级别"><el-select v-model="form.level"><el-option label="严重" value="critical" /><el-option label="警告" value="warning" /><el-option label="信息" value="info" /></el-select></el-form-item></div><el-form-item label="说明"><el-input v-model="form.description" /></el-form-item><el-checkbox v-model="form.enabled">启用规则</el-checkbox></el-form>
      <template #footer><el-button @click="dialog=false">{{ !form.editable ? '关闭' : '取消' }}</el-button><el-button v-if="form.editable" type="primary" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { deleteRule, getRules, updateRule, updateRulesBatch } from '@/api/rules'

const categories=[{value:'power',label:'开关机与电源'},{value:'storage',label:'SD 卡与存储'},{value:'recording',label:'录像与视频'},{value:'system',label:'系统稳定性'},{value:'connectivity',label:'连接通信'},{value:'feature',label:'设备功能'},{value:'tool',label:'辅助工具'}]
const levelOptions=[{value:'',label:'全部'},{value:'critical',label:'严重'},{value:'warning',label:'警告'},{value:'info',label:'信息'}]
const rules=ref([]);const loading=ref(false);const batchLoading=ref(false);const tableRef=ref(null);const selection=ref([]);const search=ref('');const category=ref('');const level=ref('');const dialog=ref(false);const form=reactive({id:0,name:'',category:'system',keyword:'',scope:'',level:'warning',enabled:true,description:'',editable:true,scenario_count:0})
const enabledCount=computed(()=>rules.value.filter(item=>item.enabled).length);const criticalCount=computed(()=>rules.value.filter(item=>item.level==='critical').length);const usedCount=computed(()=>rules.value.filter(item=>item.scenario_count>0).length)
const filtered=computed(()=>{const text=search.value.trim().toLowerCase();return rules.value.filter(item=>(!category.value||item.category===category.value)&&(!level.value||item.level===level.value)&&(!text||`${item.name}${item.keyword}${item.description}`.toLowerCase().includes(text)))})
const levelCount=(value)=>value?rules.value.filter(item=>item.level===value).length:rules.value.length
const categoryLabel=(value)=>categories.find(item=>item.value===value)?.label||value;const levelLabel=(value)=>({critical:'严重',warning:'警告',info:'信息'})[value]||value;const levelType=(value)=>({critical:'danger',warning:'warning',info:'info'})[value]||'info'
const sourceLabel=(value)=>({keyword_document:'关键字文档',admin_keyword_upload:'管理员上传',derived:'关联分析',system:'系统',manual:'手动'})[value]||value||'手动'
async function load(){loading.value=true;try{rules.value=await getRules()}finally{loading.value=false}}
function openEdit(rule){Object.assign(form,rule);dialog.value=true}
async function save(){if(!form.name.trim()||!form.keyword.trim()){ElMessage.warning('请填写名称和关键字');return}await updateRule(form.id,{...form});dialog.value=false;ElMessage.success('规则已保存');await load()}
async function saveExisting(rule){await updateRule(rule.id,rule);ElMessage.success('状态已保存')}
async function batchSetEnabled(enabled){batchLoading.value=true;try{await updateRulesBatch(selection.value.map(item=>item.id),enabled);ElMessage.success(`已${enabled?'启用':'停用'} ${selection.value.length} 条规则`);clearSelection();await load()}finally{batchLoading.value=false}}
function clearSelection(){tableRef.value?.clearSelection();selection.value=[]}
async function remove(rule){await ElMessageBox.confirm(`确定删除“${rule.name}”吗？`,'删除规则',{type:'warning'});await deleteRule(rule.id);await load()}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page h1{margin:0;font-size:22px}.page header p{margin:5px 0 0;color:#7a8493;font-size:13px}.summary{display:flex;margin-bottom:16px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.summary div{display:flex;min-width:150px;flex-direction:column;gap:5px;padding:14px 20px;border-right:1px solid #edf0f3}.summary span{color:#8a94a3;font-size:11px}.summary strong{font-size:20px}.summary .danger{color:#d95858}.panel{padding:17px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.filters{display:flex;gap:10px;margin-bottom:10px}.filters .el-input{width:min(400px,50%)}.filters .el-select{width:170px}.level-filter{display:flex;align-items:center;gap:0;margin-bottom:14px;padding-bottom:12px;border-bottom:1px solid #edf0f3}.level-filter>span{margin-right:12px;color:#7b8798;font-size:12px}.level-filter button{display:inline-flex;height:30px;align-items:center;gap:6px;padding:0 11px;border:1px solid #dfe4eb;border-right:0;background:#fff;color:#59667a;font-size:12px;cursor:pointer}.level-filter button:nth-child(2){border-radius:5px 0 0 5px}.level-filter button:last-child{border-right:1px solid #dfe4eb;border-radius:0 5px 5px 0}.level-filter button:hover{background:#f5f8fc;color:#245f9f}.level-filter button.active{position:relative;z-index:1;border-color:#8eb8e7;background:#eaf3fd;color:#1f66ad}.level-filter button.active+button{border-left-color:#8eb8e7}.level-filter i{width:7px;height:7px;border-radius:50%}.level-filter i.critical{background:#d64e4e}.level-filter i.warning{background:#d9942e}.level-filter i.info{background:#6190c5}.level-filter em{min-width:18px;padding:1px 5px;border-radius:9px;background:#f0f2f5;color:#7a8493;font-size:10px;font-style:normal;text-align:center}.level-filter button.active em{background:#fff;color:#2a69aa}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.form-grid .el-select{width:100%}.muted{color:#9aa3af;font-size:12px}@media(max-width:650px){.summary{display:grid;grid-template-columns:repeat(2,1fr)}.summary div{min-width:0}.filters{flex-wrap:wrap}.filters .el-input,.filters .el-select{width:100%}.level-filter{overflow-x:auto}.level-filter>span{display:none}.level-filter button{flex:0 0 auto}.form-grid{grid-template-columns:1fr}}
.batch-bar{display:flex;min-height:44px;align-items:center;gap:8px;margin:-2px 0 12px;padding:7px 10px;border:1px solid #cfe0f5;border-radius:5px;background:#f3f8fe;color:#5d6c7e;font-size:12px}.batch-bar span{margin-right:auto}.batch-bar strong{color:#2f72c4}@media(max-width:650px){.batch-bar{flex-wrap:wrap}.batch-bar span{width:100%}}

/* Observatory glass theme. */
.page {
  position: relative;
  height: 100%;
  padding: 28px clamp(18px, 2.4vw, 34px) 42px;
  overflow-x: hidden;
  overflow-y: auto;
  background:
    radial-gradient(circle at 62% -10%, rgba(55, 197, 221, .11), transparent 40%),
    radial-gradient(circle at 8% 38%, rgba(41, 91, 149, .08), transparent 34%),
    #080d16;
  color: #e8f0f5;
  scrollbar-color: rgba(95, 208, 227, .4) transparent;
  scrollbar-width: thin;
}
.page::after { position: absolute; z-index: 0; inset: 0; pointer-events: none; content: ''; background: linear-gradient(112deg, transparent 20%, rgba(95,217,237,.035) 52%, transparent 82%); }
.page > * { position: relative; z-index: 1; }
.page > header { align-items: center; margin-bottom: 20px; }
.page h1 { color: #f4f8fb; font-size: clamp(26px,2.2vw,34px); }
.page header p { color: #91a5b5; }
.summary { gap: 14px; border: 0; background: transparent; }
.summary div {
  position: relative;
  min-height: 94px;
  flex: 1;
  justify-content: center;
  border: 1px solid rgba(218,237,244,.18);
  border-radius: 13px;
  background: rgba(35,45,55,.62);
  box-shadow: inset 0 1px rgba(255,255,255,.08), 0 16px 38px rgba(0,0,0,.18);
  
  transition: transform .24s cubic-bezier(.4,0,.2,1), border-color .24s cubic-bezier(.4,0,.2,1), box-shadow .24s cubic-bezier(.4,0,.2,1);
}
.summary div:hover { border-color: rgba(89,214,234,.48); box-shadow: inset 0 1px rgba(255,255,255,.1), 0 18px 38px rgba(0,0,0,.24), 0 0 22px rgba(42,185,210,.1); transform: translateY(-2px); }
.summary span { color: #92a7b4; font-size: 12px; }
.summary strong { color: #f0f6f8; font-size: 28px; }
.summary .danger { color: #ff7887; text-shadow: 0 0 14px rgba(255,82,104,.2); }
.panel {
  padding: 18px;
  border: 1px solid rgba(218,237,244,.2);
  border-radius: 16px;
  background: rgba(29,38,47,.67);
  box-shadow: inset 0 1px rgba(255,255,255,.09), 0 22px 52px rgba(0,0,0,.25);
  
}
.filters { margin-bottom: 14px; }
.filters :deep(.el-input__wrapper), .filters :deep(.el-select__wrapper) { border: 1px solid rgba(211,233,240,.17); background: rgba(7,14,22,.6)!important; box-shadow: inset 0 1px rgba(255,255,255,.04); }
.filters :deep(.el-input__wrapper:hover), .filters :deep(.el-select__wrapper:hover) { border-color: rgba(79,213,235,.56); }
.level-filter { border-bottom-color: rgba(211,233,240,.12); }
.level-filter > span { color: #8da2af; }
.level-filter button { border-color: rgba(211,233,240,.13); background: rgba(7,14,22,.45); color: #93a6b3; transition: background .2s cubic-bezier(.4,0,.2,1), color .2s cubic-bezier(.4,0,.2,1); }
.level-filter button:last-child { border-right-color: rgba(211,233,240,.13); }
.level-filter button:hover { background: rgba(46,138,158,.16); color: #c7f4fa; }
.level-filter button.active { border-color: rgba(57,205,227,.56); background: rgba(20,133,153,.3); color: #a7f0f8; box-shadow: 0 0 16px rgba(39,196,219,.11); }
.level-filter button.active + button { border-left-color: rgba(57,205,227,.56); }
.level-filter em, .level-filter button.active em { background: rgba(255,255,255,.07); color: #a7bbc5; }
.batch-bar { border-color: rgba(62,205,226,.3); background: rgba(25,109,126,.22); color: #b0c6cf; }
.batch-bar strong { color: #7be7f4; }
.panel :deep(.el-table) { --el-table-bg-color: transparent; --el-table-tr-bg-color: transparent; --el-table-header-bg-color: rgba(255,255,255,.07); --el-table-row-hover-bg-color: rgba(25,132,153,.18); --el-table-border-color: rgba(216,236,243,.12); --el-table-text-color: #cbd8de; --el-table-header-text-color: #aebdc5; background: transparent; font-family: inherit; }
.panel :deep(.el-table::before) { background: rgba(216,236,243,.14); }
.panel :deep(.el-table th.el-table__cell) { height: 43px; background: rgba(255,255,255,.065); font-size: 12px; }
.panel :deep(.el-table td.el-table__cell) { height: 48px; border-bottom-color: rgba(216,236,243,.1); background: transparent; }
.panel :deep(.el-table__row) { position: relative; animation: rule-row-in .4s cubic-bezier(.4,0,.2,1) both; }
.panel :deep(.el-table__row:nth-child(2)) { animation-delay: .04s; }
.panel :deep(.el-table__row:nth-child(3)) { animation-delay: .08s; }
.panel :deep(.el-table__row:nth-child(4)) { animation-delay: .12s; }
.panel :deep(.el-table__row:nth-child(5)) { animation-delay: .16s; }
.panel :deep(.el-table__body tr:hover > td.el-table__cell) { background: rgba(25,132,153,.18)!important; }
.panel :deep(.el-table__body tr:hover > td.el-table__cell:first-child) { box-shadow: inset 2px 0 #32cce2; }
.panel :deep(.el-tag) { border-color: rgba(174,209,219,.25); background: rgba(255,255,255,.055); color: #b8cad2; }
.panel :deep(.el-tag--danger) { border-color: rgba(255,101,119,.35); background: rgba(135,38,53,.18); color: #ff909c; }
.panel :deep(.el-tag--warning) { border-color: rgba(245,177,67,.32); background: rgba(125,82,21,.2); color: #f9c66b; }
.panel :deep(.el-tag--primary) { border-color: rgba(75,206,226,.32); background: rgba(22,109,127,.22); color: #8deaf5; }
.panel :deep(.el-button.is-link) { color: #73ddeb; }
.panel :deep(.el-button--danger.is-link) { color: #ff8290; }
.muted { color: #708792; }
@keyframes rule-row-in { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 760px) { .page { padding: 18px 14px 32px; } .summary { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); } .summary div { min-height: 78px; } }
@media (prefers-reduced-motion: reduce) { .summary div, .panel :deep(.el-table__row) { animation: none; transition: none; } }
</style>
