<template>
  <div class="page">
    <header><div><h1>解析规则</h1><p>规则数据保存在 PostgreSQL</p></div><el-button type="primary" :icon="Plus" @click="openCreate">新增规则</el-button></header>
    <div class="summary"><div><span>规则总数</span><strong>{{ rules.length }}</strong></div><div><span>已启用</span><strong>{{ enabledCount }}</strong></div><div><span>严重级别</span><strong class="danger">{{ criticalCount }}</strong></div></div>
    <section class="panel">
      <div class="filters"><el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索名称、关键字或说明" /><el-select v-model="category" clearable placeholder="全部分类"><el-option v-for="item in categories" :key="item.value" :label="item.label" :value="item.value" /></el-select><el-button :icon="Refresh" :loading="loading" @click="load" /></div>
      <el-table v-loading="loading" :data="filtered">
        <el-table-column prop="name" label="规则名称" min-width="170" />
        <el-table-column prop="category" label="分类" width="110"><template #default="scope">{{ categoryLabel(scope.row.category) }}</template></el-table-column>
        <el-table-column prop="keyword" label="关键字" min-width="260" show-overflow-tooltip />
        <el-table-column prop="scope" label="适用范围" min-width="120" />
        <el-table-column prop="level" label="级别" width="90"><template #default="scope"><el-tag :type="levelType(scope.row.level)" effect="plain">{{ levelLabel(scope.row.level) }}</el-tag></template></el-table-column>
        <el-table-column label="启用" width="80"><template #default="scope"><el-switch v-model="scope.row.enabled" @change="saveExisting(scope.row)" /></template></el-table-column>
        <el-table-column label="操作" width="120"><template #default="scope"><el-button link type="primary" @click="openEdit(scope.row)">编辑</el-button><el-button link type="danger" @click="remove(scope.row)">删除</el-button></template></el-table-column>
        <template #empty><el-empty description="数据库中暂无解析规则" /></template>
      </el-table>
    </section>

    <el-dialog v-model="dialog" :title="form.id ? '编辑规则' : '新增规则'" width="560px">
      <el-form label-position="top"><div class="form-grid"><el-form-item label="规则名称"><el-input v-model="form.name" /></el-form-item><el-form-item label="分类"><el-select v-model="form.category"><el-option v-for="item in categories" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></div><el-form-item label="关键字"><el-input v-model="form.keyword" type="textarea" :rows="3" /></el-form-item><div class="form-grid"><el-form-item label="适用范围"><el-input v-model="form.scope" /></el-form-item><el-form-item label="级别"><el-select v-model="form.level"><el-option label="严重" value="critical" /><el-option label="警告" value="warning" /><el-option label="信息" value="info" /></el-select></el-form-item></div><el-form-item label="说明"><el-input v-model="form.description" /></el-form-item><el-checkbox v-model="form.enabled">启用规则</el-checkbox></el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search } from '@element-plus/icons-vue'
import { addRule, deleteRule, getRules, updateRule } from '@/api/rules'

const categories=[{value:'power',label:'开关机与电源'},{value:'storage',label:'SD 卡与存储'},{value:'recording',label:'录像与视频'},{value:'system',label:'系统稳定性'},{value:'connectivity',label:'连接通信'},{value:'feature',label:'设备功能'},{value:'tool',label:'辅助工具'}]
const rules=ref([]);const loading=ref(false);const search=ref('');const category=ref('');const dialog=ref(false);const form=reactive({id:0,name:'',category:'system',keyword:'',scope:'',level:'warning',enabled:true,description:''})
const enabledCount=computed(()=>rules.value.filter(item=>item.enabled).length);const criticalCount=computed(()=>rules.value.filter(item=>item.level==='critical').length)
const filtered=computed(()=>{const text=search.value.trim().toLowerCase();return rules.value.filter(item=>(!category.value||item.category===category.value)&&(!text||`${item.name}${item.keyword}${item.description}`.toLowerCase().includes(text)))})
const categoryLabel=(value)=>categories.find(item=>item.value===value)?.label||value;const levelLabel=(value)=>({critical:'严重',warning:'警告',info:'信息'})[value]||value;const levelType=(value)=>({critical:'danger',warning:'warning',info:'info'})[value]||'info'
async function load(){loading.value=true;try{rules.value=await getRules()}finally{loading.value=false}}
function reset(){Object.assign(form,{id:0,name:'',category:'system',keyword:'',scope:'',level:'warning',enabled:true,description:''})}
function openCreate(){reset();dialog.value=true}function openEdit(rule){Object.assign(form,rule);dialog.value=true}
async function save(){if(!form.name.trim()||!form.keyword.trim()){ElMessage.warning('请填写名称和关键字');return}if(form.id)await updateRule(form.id,{...form});else await addRule({...form,id:undefined});dialog.value=false;ElMessage.success('规则已保存');await load()}
async function saveExisting(rule){await updateRule(rule.id,rule);ElMessage.success('状态已保存')}
async function remove(rule){await ElMessageBox.confirm(`确定删除“${rule.name}”吗？`,'删除规则',{type:'warning'});await deleteRule(rule.id);await load()}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page h1{margin:0;font-size:22px}.page header p{margin:5px 0 0;color:#7a8493;font-size:13px}.summary{display:flex;margin-bottom:16px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.summary div{display:flex;min-width:150px;flex-direction:column;gap:5px;padding:14px 20px;border-right:1px solid #edf0f3}.summary span{color:#8a94a3;font-size:11px}.summary strong{font-size:20px}.summary .danger{color:#d95858}.panel{padding:17px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.filters{display:flex;gap:10px;margin-bottom:14px}.filters .el-input{width:min(400px,50%)}.filters .el-select{width:170px}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.form-grid .el-select{width:100%}@media(max-width:650px){.summary{display:grid;grid-template-columns:repeat(3,1fr)}.summary div{min-width:0}.filters{flex-wrap:wrap}.filters .el-input,.filters .el-select{width:100%}.form-grid{grid-template-columns:1fr}}
</style>
