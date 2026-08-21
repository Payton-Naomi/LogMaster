<template>
  <div class="admin-page">
    <div v-if="false" class="checking-state">
      <el-icon class="is-loading"><Loading /></el-icon><span>正在验证管理员状态</span>
    </div>

      <header class="page-heading">
        <div><div class="eyebrow">{{ selfServiceMode ? '权限自服务' : '系统管理' }}</div><h1>{{ consoleTitle(access.role) }}</h1><p>{{ selfServiceMode ? '提交自己的权限变更申请，并随时查看审批进度' : '集中维护当前角色有权管理的平台配置' }}</p></div>
        <div class="heading-actions">
          <el-button :icon="List" :type="activeModule === 'runtime_logs' ? 'primary' : 'default'" @click="toggleRuntimeLogs">{{ activeModule === 'runtime_logs' ? '返回控制台' : '运行日志' }}</el-button>
          <el-tag effect="plain" :type="roleTagType(access.role)">{{ roleLabel(access.role) }}</el-tag>
        </div>
      </header>

      <section class="module-grid" aria-label="管理员功能">
        <button v-for="module in modules" :key="module.key" type="button" class="module-card" :class="{ active: module.key === activeModule, disabled: !module.ready }" @click="selectModule(module)">
          <span class="module-icon"><el-icon><component :is="module.icon" /></el-icon></span>
          <span class="module-copy"><strong>{{ module.name }}</strong><small>{{ module.description }}</small></span>
          <el-tag v-if="!module.ready" type="info" effect="plain" size="small">待建设</el-tag>
          <el-icon v-else class="module-arrow"><ArrowRight /></el-icon>
        </button>
      </section>

      <section v-if="activeModule === 'projects'" class="project-workspace">
        <div class="section-heading">
          <div><h2>项目管理</h2><p>维护日志上传可选项目及其产品属性</p></div>
          <div class="section-actions"><el-button :icon="Setting" @click="optionDialogVisible = true">类型与阶段</el-button><el-button type="primary" :icon="Plus" @click="openCreate">新建项目</el-button></div>
        </div>

        <div class="summary-row">
          <div><span>项目总数</span><strong>{{ projects.length }}</strong></div>
          <div><span>车载线</span><strong>{{ projectStats.vehicle }}</strong></div>
          <div><span>宠物线</span><strong>{{ projectStats.pet }}</strong></div>
          <div><span>安防线</span><strong>{{ projectStats.security }}</strong></div>
        </div>

        <div class="toolbar">
          <el-input v-model="keyword" clearable :prefix-icon="Search" placeholder="搜索项目名称或说明" />
          <el-select v-model="lineFilter" clearable placeholder="全部产线">
            <el-option v-for="item in projectOptions.lines" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
          <el-select v-model="typeFilter" clearable placeholder="全部产品类型">
            <el-option v-for="item in projectOptions.types" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
          <el-select v-model="stageFilter" clearable placeholder="全部阶段">
            <el-option v-for="item in projectOptions.stages" :key="item.code" :label="item.name" :value="item.code" />
          </el-select>
          <span class="result-count">共 {{ filteredProjects.length }} 项</span>
        </div>

        <el-table v-loading="projectsLoading" :data="filteredProjects" class="desktop-table" row-key="id">
          <el-table-column label="项目" min-width="170">
            <template #default="scope"><div class="project-name"><span>{{ scope.row.name.slice(0, 2) }}</span><div><strong>{{ scope.row.name }}</strong><small>{{ scope.row.description || '暂无说明' }}</small></div></div></template>
          </el-table-column>
          <el-table-column label="产品线" width="120"><template #default="scope">{{ lineLabel(scope.row.product_line) }}</template></el-table-column>
          <el-table-column label="产品类型" width="150"><template #default="scope">{{ typeLabel(scope.row.product_type) }}</template></el-table-column>
          <el-table-column label="当前阶段" width="130"><template #default="scope"><el-tag :type="stageTagType(scope.row.stage)" effect="plain">{{ stageLabel(scope.row.stage) }}</el-tag></template></el-table-column>
          <el-table-column label="更新时间" width="175"><template #default="scope">{{ formatDate(scope.row.updated_at) }}</template></el-table-column>
          <el-table-column label="操作" width="112" align="center">
            <template #default="scope"><div class="row-actions"><el-tooltip content="编辑项目"><el-button :icon="Edit" text circle @click="openEdit(scope.row)" /></el-tooltip><el-tooltip content="删除项目"><el-button :icon="Delete" text circle type="danger" @click="removeProject(scope.row)" /></el-tooltip></div></template>
          </el-table-column>
          <template #empty><el-empty description="没有符合条件的项目" :image-size="72" /></template>
        </el-table>

        <div v-loading="projectsLoading" class="mobile-list">
          <article v-for="project in filteredProjects" :key="project.id" class="project-card">
            <div><strong>{{ project.name }}</strong><el-tag :type="stageTagType(project.stage)" effect="plain" size="small">{{ stageLabel(project.stage) }}</el-tag></div>
            <p>{{ typeLabel(project.product_type) }} · {{ lineLabel(project.product_line) }}</p><small>{{ project.description || '暂无说明' }}</small>
            <footer><span>{{ formatDate(project.updated_at) }}</span><div><el-button :icon="Edit" text circle @click="openEdit(project)" /><el-button :icon="Delete" text circle type="danger" @click="removeProject(project)" /></div></footer>
          </article>
          <el-empty v-if="!projectsLoading && !filteredProjects.length" description="没有符合条件的项目" :image-size="72" />
        </div>
      </section>

      <section v-else-if="activeModule === 'users'" class="project-workspace settings-workspace">
        <div class="section-heading">
          <div><h2>用户权限</h2><p>基于飞书账号分配平台角色，权限修改后立即生效</p></div>
          <el-tag type="danger" effect="plain">仅超级管理员可修改</el-tag>
        </div>
        <div class="role-summary">
          <div v-for="item in roleOptions" :key="item.value"><span>{{ item.label }}</span><strong>{{ userRoleCount(item.value) }}</strong></div>
        </div>
        <div class="toolbar user-toolbar">
          <el-input v-model="userKeyword" clearable :prefix-icon="Search" placeholder="搜索姓名、邮箱或飞书 ID" />
          <el-select v-model="userRoleFilter" clearable placeholder="全部等级"><el-option v-for="item in roleOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
          <span class="result-count">共 {{ filteredUsers.length }} 人</span>
        </div>
        <el-table v-loading="usersLoading" :data="filteredUsers" row-key="id">
          <el-table-column label="用户" min-width="190">
            <template #default="scope"><div class="user-identity"><span>{{ (scope.row.name || '?').slice(0, 1) }}</span><div><strong>{{ scope.row.name || '未命名用户' }} <em v-if="scope.row.is_current">当前账号</em></strong><small>{{ scope.row.email || '未绑定邮箱' }}</small></div></div></template>
          </el-table-column>
          <el-table-column prop="feishu_open_id" label="飞书 ID" min-width="190" show-overflow-tooltip />
          <el-table-column label="飞书职务" min-width="130"><template #default="scope">{{ scope.row.job_title || '-' }}</template></el-table-column>
          <el-table-column label="当前等级" width="130"><template #default="scope"><el-tag :type="roleTagType(scope.row.role)" effect="plain">{{ roleLabel(scope.row.role) }}</el-tag></template></el-table-column>
          <el-table-column label="权限说明" min-width="210"><template #default="scope"><span class="role-description">{{ roleDescription(scope.row.role) }}</span></template></el-table-column>
          <el-table-column label="调整等级" width="160" align="center">
            <template #default="scope"><el-select :model-value="scope.row.role" :loading="roleSavingId === scope.row.id" :disabled="roleSavingId === scope.row.id || isOnlyCurrentSuperAdmin(scope.row)" @change="changeUserRole(scope.row, $event)"><el-option v-for="item in roleOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></template>
          </el-table-column>
          <el-table-column label="操作" width="105" align="center"><template #default="scope"><el-button v-if="scope.row.role_source === 'manual'" text type="primary" @click="restoreUserRole(scope.row)">恢复自动</el-button><span v-else>-</span></template></el-table-column>
          <template #empty><el-empty description="没有符合条件的用户" :image-size="72" /></template>
        </el-table>
      </section>

      <section v-else-if="activeModule === 'runtime_logs'" class="project-workspace settings-workspace">
        <div class="section-heading"><div><h2>运行日志</h2><p>查询上传、解析、解压和登录等操作及异常原因</p></div><el-button :icon="Search" @click="loadRuntimeLogs">刷新</el-button></div>
        <div class="toolbar"><el-select v-model="runtimeModuleFilter" clearable placeholder="全部模块"><el-option label="上传" value="upload"/><el-option label="解析" value="parsing"/><el-option label="解压" value="archive"/><el-option label="登录" value="auth"/></el-select><el-select v-model="runtimeStatusFilter" clearable placeholder="全部状态"><el-option label="成功" value="success"/><el-option label="失败" value="failed"/><el-option label="警告" value="warning"/></el-select></div>
        <el-table v-loading="runtimeLogsLoading" :data="filteredRuntimeLogs" row-key="id">
          <el-table-column label="时间" width="170"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
          <el-table-column prop="module" label="模块" width="90"/><el-table-column prop="event" label="事件" min-width="150"/>
          <el-table-column label="状态" width="90"><template #default="scope"><el-tag :type="scope.row.status === 'failed' ? 'danger' : scope.row.status === 'warning' ? 'warning' : 'success'">{{ scope.row.status }}</el-tag></template></el-table-column>
          <el-table-column prop="message" label="原因/说明" min-width="230" show-overflow-tooltip/><el-table-column prop="task_id" label="任务 ID" min-width="150" show-overflow-tooltip/><el-table-column prop="query_code" label="查询码" width="130"/><el-table-column prop="owner_name" label="用户" width="110"/>
        </el-table>
      </section>

      <section v-else-if="activeModule === 'capacity'" class="project-workspace settings-workspace">
        <div class="section-heading">
          <div><h2>资源与配额</h2><p>集中管理日志上传规模和 AI 分析用量</p></div>
          <el-tag type="success" effect="plain">全局配置</el-tag>
        </div>
        <div class="resource-settings-grid">
          <section class="resource-setting-section">
            <div class="resource-setting-heading"><strong>上传限制</strong><span>限制每批上传的文件总量</span></div>
            <div class="capacity-overview">
              <div><span>当前总容量</span><strong>{{ formatSize(capacityForm.max_upload_bytes) }}</strong></div>
              <div><span>当前文件数</span><strong>{{ capacityForm.max_files_per_upload }} 个</strong></div>
              <div><span>最近更新</span><strong class="date-value">{{ formatDate(capacityUpdatedAt) }}</strong></div>
            </div>
            <el-form class="capacity-form" label-position="top" @submit.prevent="saveCapacity">
              <el-form-item label="单批总容量（MB）" required>
                <el-input-number v-model="capacityMegabytes" :min="1" :max="102400" :step="100" controls-position="right" />
                <span class="field-help">允许范围 1 MB 至 100 GB</span>
              </el-form-item>
              <el-form-item label="单批文件数量" required>
                <el-input-number v-model="capacityForm.max_files_per_upload" :min="1" :max="500" controls-position="right" />
                <span class="field-help">允许范围 1 至 500 个文件</span>
              </el-form-item>
              <el-button type="primary" :loading="capacitySaving" @click="saveCapacity">保存上传限制</el-button>
            </el-form>
          </section>

          <section class="resource-setting-section ai-quota-section">
            <div class="resource-setting-heading"><strong>AI 分析配额</strong><span>控制 AI 分析范围和每日 Token 成本</span></div>
            <div class="ai-quota-overview">
              <div><span>单文件最大输出</span><strong>{{ aiSettingsForm.max_tokens_per_file.toLocaleString() }} Token</strong></div>
              <div><span>每用户每日 Token</span><strong>{{ formatTokenQuota(aiSettingsForm.daily_token_quota) }}</strong></div>
            </div>
            <el-form class="capacity-form" label-position="top" @submit.prevent="saveAISettings">
              <el-form-item label="单个文件最大输出 Token" required>
                <el-input-number v-model="aiSettingsForm.max_tokens_per_file" :min="1" :max="1000000" :step="1000" controls-position="right" />
                <span class="field-help">限制单个文件 AI 模型的最大输出，关键字分析不受影响</span>
              </el-form-item>
              <el-form-item label="每用户每日 Token 配额" required>
                <el-input-number v-model="aiSettingsForm.daily_token_quota" :min="0" :step="100000" controls-position="right" />
                <span class="field-help">填 0 表示不限制；额度用完后当天仅进行关键字分析</span>
              </el-form-item>
              <el-button type="primary" :loading="aiSettingsSaving" @click="saveAISettings">保存 AI 配额</el-button>
            </el-form>
          </section>
        </div>
      </section>

      <section v-else-if="activeModule === 'approvals'" class="project-workspace approval-workspace">
        <div class="section-heading">
          <div><h2>权限申请</h2><p>申请工作所需权限，审批通过后立即生效</p></div>
          <el-tag :type="roleTagType(approvalAccess.current_role)" effect="plain">当前：{{ roleLabel(approvalAccess.current_role) }}</el-tag>
        </div>

        <div v-if="access.role === 'user'" class="permission-guide">
          <div class="permission-guide-heading"><strong>你可以申请的权限</strong><span>按工作需要选择，权限越高可用功能越多</span></div>
          <div class="permission-guide-grid">
            <article v-for="item in roleBenefits" :key="item.role" class="permission-guide-item" :class="{ current: item.role === access.role }">
              <div><el-tag :type="roleTagType(item.role)" effect="plain">{{ roleLabel(item.role) }}</el-tag><span v-if="item.role === access.role">当前权限</span></div>
              <strong>{{ item.summary }}</strong>
              <ul><li v-for="feature in item.features" :key="feature">{{ feature }}</li></ul>
            </article>
          </div>
        </div>

        <div v-if="approvalAccess.can_apply" class="approval-apply">
          <div class="approval-apply-copy"><strong>申请权限</strong><span v-if="pendingOwnRequest">当前申请正在等待审批</span><span v-else>选择需要的权限，并简单说明用途</span></div>
          <el-form class="approval-form" label-position="top" @submit.prevent="submitApproval">
            <el-form-item label="申请等级" required><el-select v-model="approvalForm.requested_role" :disabled="Boolean(pendingOwnRequest)" placeholder="选择目标等级"><el-option v-for="item in availableApprovalRoles" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
            <el-form-item label="申请原因" required><el-input v-model="approvalForm.reason" :disabled="Boolean(pendingOwnRequest)" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="简单说明需要使用的功能" /></el-form-item>
            <div class="approval-form-actions"><el-button v-if="pendingOwnRequest" type="danger" plain :loading="approvalActionId === pendingOwnRequest.id" @click="cancelApproval(pendingOwnRequest)">撤回当前申请</el-button><el-button v-else type="primary" :loading="approvalSubmitting" :disabled="!approvalForm.requested_role || !approvalForm.reason.trim()" @click="submitApproval">提交申请</el-button></div>
          </el-form>
        </div>
        <el-alert v-else title="超级管理员拥有全部权限，无需提交权限变更申请。" type="info" :closable="false" show-icon />

        <div class="approval-list-heading"><div><strong>{{ approvalAccess.can_review ? '全部申请' : '我的申请' }}</strong><span>{{ approvalAccess.can_review ? '管理员可审批其他用户提交的待处理申请' : '这里会保留你的申请和审批结果' }}</span></div><el-segmented v-model="approvalStatusFilter" :options="approvalStatusOptions" size="small" /></div>
        <el-table v-loading="approvalsLoading" :data="filteredApprovalRequests" row-key="id" class="approval-table">
          <el-table-column v-if="approvalAccess.can_review" label="申请人" min-width="150"><template #default="scope"><div class="approval-applicant"><strong>{{ scope.row.applicant_name || '未命名用户' }}</strong><small>{{ scope.row.applicant_email || scope.row.applicant_open_id }}</small></div></template></el-table-column>
          <el-table-column label="权限变更" min-width="180"><template #default="scope"><div class="role-change"><el-tag :type="roleTagType(scope.row.current_role)" effect="plain" size="small">{{ roleLabel(scope.row.current_role) }}</el-tag><el-icon><ArrowRight /></el-icon><el-tag :type="roleTagType(scope.row.requested_role)" effect="plain" size="small">{{ roleLabel(scope.row.requested_role) }}</el-tag></div></template></el-table-column>
          <el-table-column prop="reason" label="申请原因" min-width="220" show-overflow-tooltip />
          <el-table-column label="状态" width="100"><template #default="scope"><el-tag :type="approvalStatusType(scope.row.status)" effect="light" size="small">{{ approvalStatusLabel(scope.row.status) }}</el-tag></template></el-table-column>
          <el-table-column label="审批结果" min-width="180" show-overflow-tooltip><template #default="scope"><span v-if="scope.row.status === 'pending'" class="reviewer-name">等待管理员处理</span><span v-else>{{ scope.row.review_comment || '无补充说明' }}<small v-if="scope.row.reviewer_name" class="reviewer-inline"> · {{ scope.row.reviewer_name }}</small></span></template></el-table-column>
          <el-table-column label="提交时间" width="170"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
          <el-table-column v-if="approvalAccess.can_review" label="操作" width="155" align="center">
            <template #default="scope"><div v-if="scope.row.status === 'pending'" class="row-actions"><el-button size="small" type="success" plain :loading="approvalActionId === scope.row.id" :disabled="scope.row.applicant_open_id === access.open_id || (scope.row.requested_role === 'super_admin' && access.role !== 'super_admin')" @click="decideApproval(scope.row, 'approve')">通过</el-button><el-button size="small" type="danger" plain :loading="approvalActionId === scope.row.id" :disabled="scope.row.applicant_open_id === access.open_id" @click="decideApproval(scope.row, 'reject')">驳回</el-button></div><span v-else class="reviewer-name">{{ scope.row.reviewer_name || '-' }}</span></template>
          </el-table-column>
          <template #empty><el-empty description="暂无权限申请" :image-size="70" /></template>
        </el-table>

        <div class="approval-divider"></div>
        <div class="section-heading project-approval-heading">
          <div><h2>项目申请与审批</h2><p>{{ projectRequestAccess.can_review ? '审核用户提交的新项目，审核通过后项目才可用于日志上传' : '提交新项目申请，并查看管理员的审核结果' }}</p></div>
          <el-tag type="warning" effect="plain">审核后生效</el-tag>
        </div>
        <el-form v-if="!projectRequestAccess.can_review" class="project-request-form" label-position="top" @submit.prevent="submitProjectRequest">
          <div class="form-grid"><el-form-item label="项目名称" required><el-input v-model.trim="projectRequestForm.name" maxlength="128" placeholder="例如 DR2862"/></el-form-item><el-form-item label="产品线" required><el-select v-model="projectRequestForm.product_line" placeholder="请选择"><el-option v-for="item in projectOptions.lines" :key="item.code" :label="item.name" :value="item.code"/></el-select></el-form-item><el-form-item label="产品类型" required><el-select v-model="projectRequestForm.product_type" placeholder="请选择"><el-option v-for="item in projectOptions.types" :key="item.code" :label="item.name" :value="item.code"/></el-select></el-form-item><el-form-item label="阶段" required><el-select v-model="projectRequestForm.stage" placeholder="请选择"><el-option v-for="item in projectOptions.stages" :key="item.code" :label="item.name" :value="item.code"/></el-select></el-form-item></div>
          <el-form-item label="项目说明"><el-input v-model="projectRequestForm.description" type="textarea" :rows="2" maxlength="1000"/></el-form-item><el-form-item label="申请原因" required><el-input v-model="projectRequestForm.reason" type="textarea" :rows="2" maxlength="1000"/></el-form-item>
          <el-button type="primary" :loading="projectRequestSubmitting" :disabled="!projectRequestForm.name || !projectRequestForm.product_line || !projectRequestForm.product_type || !projectRequestForm.stage || !projectRequestForm.reason" @click="submitProjectRequest">提交项目申请</el-button>
        </el-form>
        <el-table v-loading="projectRequestsLoading" :data="projectRequests" row-key="id" class="request-table project-request-table">
          <el-table-column prop="name" label="项目" width="130"/><el-table-column v-if="projectRequestAccess.can_review" prop="applicant_name" label="申请人" width="110"/><el-table-column prop="reason" label="申请原因" min-width="180" show-overflow-tooltip/>
          <el-table-column label="状态" width="100"><template #default="scope"><el-tag :type="scope.row.status === 'approved' ? 'success' : scope.row.status === 'rejected' ? 'danger' : 'warning'">{{ projectRequestStatusLabel(scope.row.status) }}</el-tag></template></el-table-column>
          <el-table-column prop="review_comment" label="审核意见" min-width="160" show-overflow-tooltip/><el-table-column label="申请时间" width="170"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
          <el-table-column v-if="projectRequestAccess.can_review" label="审核" width="150"><template #default="scope"><div v-if="scope.row.status === 'pending'" class="row-actions"><el-button size="small" type="success" @click="reviewProjectRequest(scope.row,'approve')">通过</el-button><el-button size="small" type="danger" @click="reviewProjectRequest(scope.row,'reject')">驳回</el-button></div><span v-else>-</span></template></el-table-column>
          <template #empty><el-empty :description="projectRequestAccess.can_review ? '暂无项目申请' : '还没有提交项目申请'" :image-size="70" /></template>
        </el-table>
      </section>

      <section v-else-if="activeModule === 'keywords'" class="project-workspace settings-workspace">
        <div class="section-heading">
          <div><h2>研发异常关键词</h2><p>批量导入公共规则，导入后所有用户均可在解析规则页查看并参与日志分析</p></div>
          <el-tag effect="plain">{{ keywordRules.length }} 条已上传</el-tag>
        </div>
        <div class="keyword-layout">
          <el-form class="keyword-import" label-position="top" @submit.prevent="submitKeywordImport">
            <div class="form-grid">
              <el-form-item label="默认分类"><el-select v-model="keywordDefaults.category"><el-option v-for="item in keywordCategories" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
              <el-form-item label="默认级别"><el-select v-model="keywordDefaults.level"><el-option label="严重" value="critical" /><el-option label="警告" value="warning" /><el-option label="信息" value="info" /></el-select></el-form-item>
            </div>
            <el-form-item label="适用范围"><el-input v-model.trim="keywordDefaults.scope" maxlength="128" placeholder="例如 自研通用、DR2861" /></el-form-item>
            <el-form-item label="规则说明"><el-input v-model="keywordDefaults.description" type="textarea" :rows="2" maxlength="1000" show-word-limit placeholder="选填，将应用于 TXT 中的全部关键词" /></el-form-item>
            <input ref="keywordFileInput" type="file" hidden accept=".txt,.csv" @change="selectKeywordFile">
            <button type="button" class="keyword-drop" @click="keywordFileInput?.click()">
              <el-icon><DocumentAdd /></el-icon>
              <span><strong>{{ keywordFile?.name || '选择关键词文件' }}</strong><small>{{ keywordFile ? formatSize(keywordFile.size) : 'TXT 每行一个关键词；CSV 支持中英文表头，最大 2 MB' }}</small></span>
            </button>
            <el-button type="primary" :loading="keywordImporting" :disabled="!keywordFile" @click="submitKeywordImport">导入到解析规则</el-button>
          </el-form>
          <div class="format-guide">
            <strong>CSV 可用列</strong>
            <code>name, keyword, category, level, scope, description</code>
            <p>仅 keyword 为必填列。CSV 行内填写的分类、级别和范围会覆盖左侧默认值；重复的关键词会更新原规则。</p>
          </div>
        </div>
        <el-table v-loading="keywordRulesLoading" :data="keywordRules" class="keyword-table" row-key="id">
          <el-table-column prop="name" label="规则名称" min-width="170" show-overflow-tooltip />
          <el-table-column prop="keyword" label="关键词" min-width="220" show-overflow-tooltip />
          <el-table-column label="分类" width="100"><template #default="scope">{{ categoryLabel(scope.row.category) }}</template></el-table-column>
          <el-table-column label="级别" width="90"><template #default="scope"><el-tag :type="levelTagType(scope.row.level)" effect="plain" size="small">{{ levelLabel(scope.row.level) }}</el-tag></template></el-table-column>
          <el-table-column prop="scope" label="适用范围" min-width="120" show-overflow-tooltip />
          <el-table-column label="操作" width="70" align="center"><template #default="scope"><el-tooltip content="删除规则"><el-button :icon="Delete" text circle type="danger" @click="removeKeywordRule(scope.row)" /></el-tooltip></template></el-table-column>
          <template #empty><el-empty description="还没有管理员上传的关键词规则" :image-size="70" /></template>
        </el-table>
      </section>

    <el-dialog v-model="dialogVisible" class="admin-glass-dialog" :title="editingId ? '编辑项目' : '新建项目'" width="min(520px, 92vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="saveProject">
        <div class="form-grid">
          <el-form-item label="项目名称" required><el-input v-model.trim="projectForm.name" maxlength="128" placeholder="例如 DR2862" @input="projectForm.name = projectForm.name.toUpperCase()" /></el-form-item>
          <el-form-item label="产品线" required><el-select v-model="projectForm.product_line"><el-option v-for="item in projectOptions.lines" :key="item.code" :label="item.name" :value="item.code" /></el-select></el-form-item>
          <el-form-item label="产品类型" required><el-select v-model="projectForm.product_type"><el-option v-for="item in projectOptions.types" :key="item.code" :label="item.name" :value="item.code" /></el-select></el-form-item>
          <el-form-item label="当前阶段" required><el-select v-model="projectForm.stage"><el-option v-for="item in projectOptions.stages" :key="item.code" :label="item.name" :value="item.code" /></el-select></el-form-item>
        </div>
        <el-form-item label="项目说明"><el-input v-model="projectForm.description" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="补充项目代际、平台或维护信息（可选）" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!projectForm.name || !projectForm.product_line || !projectForm.product_type || !projectForm.stage" @click="saveProject">{{ editingId ? '保存修改' : '创建项目' }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="optionDialogVisible" class="admin-glass-dialog admin-option-dialog" title="项目类型与阶段" width="min(720px, 92vw)" destroy-on-close>
      <div class="option-manager">
        <section v-for="group in optionGroups" :key="group.kind" class="option-group">
          <div class="option-heading"><div><h3>{{ group.title }}</h3><p>{{ group.description }}</p></div></div>
          <div class="option-create"><el-input v-model.trim="newOptionNames[group.kind]" maxlength="64" :placeholder="`输入${group.title}名称`" @keyup.enter="addOption(group.kind)" /><el-button :icon="Plus" :loading="optionSaving === group.kind" @click="addOption(group.kind)">添加</el-button></div>
          <div class="option-list">
            <div v-for="item in projectOptions[group.key]" :key="item.id" class="option-row">
              <span>{{ item.name }}</span><el-tag v-if="item.is_system" type="info" effect="plain" size="small">系统预置</el-tag>
              <div><el-tooltip content="修改名称"><el-button :icon="Edit" text circle @click="renameOption(item)" /></el-tooltip><el-tooltip :content="item.is_system ? '系统预置项不能删除' : '删除'"><el-button :icon="Delete" text circle type="danger" :disabled="item.is_system" @click="removeOption(item)" /></el-tooltip></div>
            </div>
          </div>
        </section>
      </div>
      <template #footer><el-button type="primary" @click="optionDialogVisible = false">完成</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, markRaw, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, Check, Delete, DocumentAdd, Edit, Key, List, Loading, Plus, Search, Setting, User, UserFilled } from '@element-plus/icons-vue'
import { cancelPermissionRequest as cancelPermissionRequestApi, createAdminProject, createPermissionRequest, createProjectOption, createProjectRequest, decidePermissionRequest, decideProjectRequest, deleteAdminKeywordRule, deleteAdminProject, deleteProjectOption, getAdminKeywordRules, getAdminProjects, getAdminUsers, getAIAnalysisSettings, getPermissionRequests, getProjectOptions, getProjectRequests, getRuntimeLogs, getUploadCapacity, importAdminKeywordRules, restoreAdminUserRole, updateAdminProject, updateAdminUserRole, updateAIAnalysisSettings, updateProjectOption, updateUploadCapacity } from '@/api/admin'
import { getCurrentUser } from '@/api/auth'

const checking = ref(false)
const unlocked = ref(true)
const access = reactive({ role: 'super_admin', permissions: rolePermissions('super_admin'), open_id: '' })
const currentUser = reactive({ name: '刘欣彤', email: '', role: 'super_admin', feishu_open_id: '' })
const selfServiceMode = ref(false)
const activeModule = ref('projects')
const previousModule = ref('projects')
const users = ref([])
const usersLoading = ref(false)
const userKeyword = ref('')
const userRoleFilter = ref('')
const roleSavingId = ref(null)
const approvalRequests = ref([])
const approvalsLoading = ref(false)
const approvalSubmitting = ref(false)
const approvalActionId = ref(null)
const approvalStatusFilter = ref('all')
const approvalAccess = reactive({ current_role: 'user', can_apply: true, can_review: false })
const approvalForm = reactive({ requested_role: 'developer', reason: '' })
const runtimeLogs = ref([]), runtimeLogsLoading = ref(false), runtimeModuleFilter = ref(''), runtimeStatusFilter = ref('')
const projectRequests = ref([]), projectRequestsLoading = ref(false), projectRequestSubmitting = ref(false)
const projectRequestAccess = reactive({ can_review: false })
const projectRequestForm = reactive({ name: '', product_line: '', product_type: '', stage: '', description: '', reason: '' })
const projects = ref([])
const projectOptions = ref({ lines: [], types: [], stages: [] })
const projectsLoading = ref(false)
const keyword = ref('')
const lineFilter = ref('')
const typeFilter = ref('')
const stageFilter = ref('')
const dialogVisible = ref(false)
const editingId = ref(null)
const saving = ref(false)
const projectForm = reactive({ name: '', product_line: 'vehicle', product_type: 'dashcam', stage: 'production', description: '' })
const optionDialogVisible = ref(false)
const optionSaving = ref('')
const newOptionNames = reactive({ type: '', stage: '' })
const capacityForm = reactive({ max_upload_bytes: 2 * 1024 * 1024 * 1024, max_files_per_upload: 100 })
const capacityUpdatedAt = ref('')
const capacitySaving = ref(false)
const aiSettingsForm = reactive({ max_tokens_per_file: 20000, daily_token_quota: 1000000 })
const aiSettingsSaving = ref(false)
const keywordFileInput = ref(null)
const keywordFile = ref(null)
const keywordImporting = ref(false)
const keywordRulesLoading = ref(false)
const keywordRules = ref([])
const keywordDefaults = reactive({ category: 'system', level: 'critical', scope: '自研通用', description: '' })
const keywordCategories = [
  { value: 'power', label: '电源' }, { value: 'storage', label: '存储' }, { value: 'recording', label: '录像' },
  { value: 'system', label: '系统' }, { value: 'connectivity', label: '连接' }, { value: 'feature', label: '功能' }, { value: 'tool', label: '工具' }
]
const optionGroups = [
  { kind: 'type', key: 'types', title: '项目类型', description: '用于描述产品形态或设备类别' },
  { kind: 'stage', key: 'stages', title: '项目阶段', description: '用于标识项目当前生命周期' }
]

const roleOptions = [
  { value: 'user', label: '普通用户' }, { value: 'developer', label: '开发' },
  { value: 'admin', label: '普通管理员' }, { value: 'super_admin', label: '超级管理员' }
]
const roleBenefits = [
  { role: 'user', summary: '上传并查看日志', features: ['上传日志并查看分析结果', '申请项目或更高权限'] },
  { role: 'developer', summary: '维护公共解析规则', features: ['包含普通用户功能', '导入和维护异常关键词'] },
  { role: 'admin', summary: '管理项目和审批', features: ['包含开发功能', '管理项目并审核申请'] },
  { role: 'super_admin', summary: '管理平台权限与配额', features: ['包含管理员功能', '管理用户权限和平台配额'] }
]
const moduleDefinitions = [
  { key: 'users', permission: 'users', name: '用户权限', description: '成员角色与数据权限', icon: markRaw(UserFilled), ready: true },
  { key: 'projects', permission: 'projects', name: '项目管理', description: '项目资料与可选列表', icon: markRaw(List), ready: true },
  { key: 'capacity', permission: 'capacity', name: '资源与配额', description: '上传与 AI 用量限制', icon: markRaw(Setting), ready: true },
  { key: 'approvals', permission: 'approvals', name: '申请与审批', description: '权限及项目申请审批', icon: markRaw(Check), ready: true },
  { key: 'keywords', permission: 'keywords', name: '异常关键词', description: '研发专属关键词上传', icon: markRaw(DocumentAdd), ready: true }
]
const modules = computed(() => moduleDefinitions.filter(module => access.permissions.includes(module.permission)))
const availableApprovalRoles = computed(() => roleOptions.filter(item => item.value !== approvalAccess.current_role))
const pendingOwnRequest = computed(() => approvalRequests.value.find(request => request.applicant_open_id === currentUser.feishu_open_id && request.status === 'pending'))
const filteredApprovalRequests = computed(() => approvalRequests.value.filter(request => approvalStatusFilter.value === 'all' || request.status === approvalStatusFilter.value))
const filteredRuntimeLogs = computed(() => runtimeLogs.value.filter(item => (!runtimeModuleFilter.value || item.module === runtimeModuleFilter.value) && (!runtimeStatusFilter.value || item.status === runtimeStatusFilter.value)))
const approvalStatusOptions = [
  { label: '全部', value: 'all' }, { label: '待审批', value: 'pending' },
  { label: '已通过', value: 'approved' }, { label: '已驳回', value: 'rejected' }
]

const capacityMegabytes = computed({
  get: () => Math.max(1, Math.round(capacityForm.max_upload_bytes / (1024 * 1024))),
  set: value => { capacityForm.max_upload_bytes = Number(value || 1) * 1024 * 1024 }
})
const projectStats = computed(() => projects.value.reduce((stats, project) => {
  if (Object.hasOwn(stats, project.product_line)) stats[project.product_line] += 1
  return stats
}, { vehicle: 0, pet: 0, security: 0 }))
const filteredProjects = computed(() => {
  const text = keyword.value.trim().toLowerCase()
  return projects.value.filter((project) => (!text || `${project.name}${project.description}`.toLowerCase().includes(text))
    && (!lineFilter.value || project.product_line === lineFilter.value)
    && (!typeFilter.value || project.product_type === typeFilter.value)
    && (!stageFilter.value || project.stage === stageFilter.value))
})
const filteredUsers = computed(() => {
  const text = userKeyword.value.trim().toLowerCase()
  return users.value.filter(user => (!text || `${user.name}${user.email}${user.feishu_open_id}`.toLowerCase().includes(text))
    && (!userRoleFilter.value || user.role === userRoleFilter.value))
})
const superAdminCount = computed(() => users.value.filter(user => user.role === 'super_admin').length)

function rolePermissions(role) {
  if (role === 'super_admin') return ['users', 'projects', 'capacity', 'approvals', 'keywords', 'runtime_logs']
  if (role === 'admin') return ['projects', 'approvals', 'keywords', 'runtime_logs']
  if (role === 'developer') return ['approvals', 'keywords', 'runtime_logs']
  return ['approvals', 'runtime_logs']
}
function optionLabel(key, value) { return projectOptions.value[key].find((item) => item.code === value)?.name || value || '-' }
function lineLabel(value) { return optionLabel('lines', value) }
function typeLabel(value) { return optionLabel('types', value) }
function stageLabel(value) { return optionLabel('stages', value) }
function stageTagType(value) { return value === 'development' ? 'warning' : value === 'production' ? 'success' : 'primary' }
function formatDate(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-' }
function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
}
function formatTokenQuota(value) {
  const quota = Number(value) || 0
  return quota === 0 ? '不限额' : `${quota.toLocaleString('zh-CN')} Token`
}
function categoryLabel(value) { return keywordCategories.find(item => item.value === value)?.label || value }
function levelLabel(value) { return ({ critical: '严重', warning: '警告', info: '信息' })[value] || value }
function levelTagType(value) { return value === 'critical' ? 'danger' : value === 'warning' ? 'warning' : 'info' }
function roleLabel(value) { return roleOptions.find(item => item.value === value)?.label || value }
function roleTagType(value) { return value === 'super_admin' ? 'danger' : value === 'admin' ? 'warning' : value === 'developer' ? 'primary' : 'info' }
function roleDescription(value) { return ({ user: '正常使用业务功能，可申请调整权限', developer: '可维护异常关键词并提交权限申请', admin: '可管理项目、异常关键词并提交权限申请', super_admin: '拥有全部管理及权限审批能力' })[value] || '-' }
function consoleTitle(value) { return ({ user: '普通用户权限申请', developer: '开发控制台', admin: '普通管理员控制台', super_admin: '超级管理员控制台' })[value] || '权限申请' }
function approvalStatusLabel(value) { return ({ pending: '待审批', approved: '已通过', rejected: '已驳回', cancelled: '已撤回' })[value] || value }
function approvalStatusType(value) { return value === 'approved' ? 'success' : value === 'rejected' ? 'danger' : value === 'cancelled' ? 'info' : 'warning' }
function projectRequestStatusLabel(value) { return ({ pending: '待审批', approved: '已通过', rejected: '已驳回', cancelled: '已撤回' })[value] || value }
function userRoleCount(role) { return users.value.filter(user => user.role === role).length }
function isOnlyCurrentSuperAdmin(user) { return user.is_current && user.role === 'super_admin' && superAdminCount.value <= 1 }
async function selectModule(module) {
  if (!module.ready) { ElMessage.info(`${module.name}将在下一阶段开放`); return }
  activeModule.value = module.key
  if (module.key === 'users') await loadUsers()
  if (module.key === 'projects') await loadProjectData()
  if (module.key === 'capacity') await Promise.all([loadCapacity(), loadAISettings()])
  if (module.key === 'approvals') await Promise.all([loadApprovals(), loadProjectRequests()])
  if (module.key === 'keywords') await loadKeywordRules()
}

async function toggleRuntimeLogs() {
  if (activeModule.value === 'runtime_logs') {
    activeModule.value = previousModule.value
    await loadActiveModule()
    return
  }
  previousModule.value = activeModule.value || modules.value[0]?.key || 'approvals'
  activeModule.value = 'runtime_logs'
  await loadRuntimeLogs()
}

async function loadRuntimeLogs(){runtimeLogsLoading.value=true;try{runtimeLogs.value=await getRuntimeLogs()||[]}finally{runtimeLogsLoading.value=false}}
async function loadProjectRequests(){projectRequestsLoading.value=true;try{const [result]=await Promise.all([getProjectRequests(),loadProjectOptions()]);projectRequests.value=result?.requests||[];projectRequestAccess.can_review=Boolean(result?.can_review)}finally{projectRequestsLoading.value=false}}
async function submitProjectRequest(){if(projectRequestSubmitting.value)return;projectRequestSubmitting.value=true;try{await createProjectRequest(projectRequestForm);Object.assign(projectRequestForm,{name:'',product_line:'',product_type:'',stage:'',description:'',reason:''});ElMessage.success('项目申请已提交，审核通过后可用于上传');await loadProjectRequests()}finally{projectRequestSubmitting.value=false}}
async function reviewProjectRequest(item,action){try{const {value}=await ElMessageBox.prompt(action==='approve'?'填写通过意见（可选）':'填写驳回原因','项目审核',{confirmButtonText:action==='approve'?'通过':'驳回',cancelButtonText:'取消',inputValidator:v=>action==='approve'||(v||'').trim()?true:'请填写驳回原因'});await decideProjectRequest(item.id,{action,comment:(value||'').trim()});ElMessage.success(action==='approve'?'项目已创建':'申请已驳回');await loadProjectRequests();if(action==='approve'&&access.permissions.includes('projects'))await loadProjects()}catch{}}

async function enterSelfService(clearSession = true) {
  if (clearSession) {
  }
  selfServiceMode.value = true
  unlocked.value = true
  Object.assign(access, { role: currentUser.role, permissions: ['approvals', 'runtime_logs'], open_id: currentUser.feishu_open_id })
  activeModule.value = 'approvals'
  await Promise.all([loadApprovals(), loadProjectRequests()])
}

async function loadApprovals() {
  approvalsLoading.value = true
  try {
    const result = await getPermissionRequests()
    approvalRequests.value = result?.requests || []
    Object.assign(approvalAccess, {
      current_role: result?.current_role || currentUser.role,
      can_apply: Boolean(result?.can_apply),
      can_review: Boolean(result?.can_review)
    })
    currentUser.role = approvalAccess.current_role
    access.role = approvalAccess.current_role
    if (!pendingOwnRequest.value && approvalForm.requested_role === approvalAccess.current_role) {
      approvalForm.requested_role = availableApprovalRoles.value[0]?.value || ''
    }
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else ElMessage.error('权限申请加载失败')
  } finally { approvalsLoading.value = false }
}

async function submitApproval() {
  if (!approvalForm.requested_role || !approvalForm.reason.trim() || approvalSubmitting.value) return
  approvalSubmitting.value = true
  try {
    await createPermissionRequest({ requested_role: approvalForm.requested_role, reason: approvalForm.reason.trim() })
    approvalForm.reason = ''
    ElMessage.success('权限申请已提交')
    await loadApprovals()
  } finally { approvalSubmitting.value = false }
}

async function cancelApproval(request) {
  try {
    await ElMessageBox.confirm('撤回后可以重新提交新的权限申请。', '撤回申请', { confirmButtonText: '确认撤回', cancelButtonText: '取消', type: 'warning' })
    approvalActionId.value = request.id
    await cancelPermissionRequestApi(request.id)
    ElMessage.success('申请已撤回')
    await loadApprovals()
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  } finally { approvalActionId.value = null }
}

async function decideApproval(request, action) {
  const approving = action === 'approve'
  try {
    const { value } = await ElMessageBox.prompt(approving ? '可填写审批说明，也可以直接通过。' : '请填写驳回原因，帮助申请人重新调整。', approving ? '通过权限申请' : '驳回权限申请', {
      confirmButtonText: approving ? '确认通过' : '确认驳回', cancelButtonText: '取消',
      inputPlaceholder: approving ? '审批说明（选填）' : '驳回原因（必填）',
      inputValidator: value => approving || (value || '').trim() ? true : '请填写驳回原因'
    })
    approvalActionId.value = request.id
    await decidePermissionRequest(request.id, { action, comment: (value || '').trim() })
    ElMessage.success(approving ? '申请已通过，用户权限已更新' : '申请已驳回')
    await loadApprovals()
    if (approvalAccess.can_review && access.permissions.includes('users')) await loadUsers()
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  } finally { approvalActionId.value = null }
}

async function loadProjects() {
  projectsLoading.value = true
  try { projects.value = await getAdminProjects() || [] }
  catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else ElMessage.error('项目列表加载失败')
  } finally { projectsLoading.value = false }
}

async function loadProjectOptions() {
  try { projectOptions.value = await getProjectOptions() || { lines: [], types: [], stages: [] } }
  catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else ElMessage.error('项目属性加载失败')
  }
}

async function loadProjectData() {
  await Promise.all([loadProjects(), loadProjectOptions()])
}

async function loadUsers() {
  usersLoading.value = true
  try { users.value = await getAdminUsers() || [] }
  catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else if (error.response?.status !== 403) ElMessage.error('用户列表加载失败')
  } finally { usersLoading.value = false }
}

async function changeUserRole(user, role) {
  if (role === user.role || roleSavingId.value) return
  try {
    await ElMessageBox.confirm(`确认将“${user.name || user.feishu_open_id}”调整为“${roleLabel(role)}”？权限将立即生效。`, '调整用户等级', { confirmButtonText: '确认调整', cancelButtonText: '取消', type: 'warning' })
    roleSavingId.value = user.id
    const updated = await updateAdminUserRole(user.id, role)
    users.value = users.value.map(item => item.id === user.id ? updated : item)
    ElMessage.success('用户等级已更新')
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  } finally { roleSavingId.value = null }
}

async function restoreUserRole(user) {
  try {
    const updated = await restoreAdminUserRole(user.id)
    users.value = users.value.map(item => item.id === user.id ? updated : item)
    ElMessage.success('已恢复按飞书职位自动分配')
  } catch (error) { if (error?.response?.status === 401) ElMessage.error('当前会话无权执行此操作') }
}

async function loadCapacity() {
  try {
    const capacity = await getUploadCapacity()
    capacityForm.max_upload_bytes = Number(capacity?.max_upload_bytes) || capacityForm.max_upload_bytes
    capacityForm.max_files_per_upload = Number(capacity?.max_files_per_upload) || capacityForm.max_files_per_upload
    capacityUpdatedAt.value = capacity?.updated_at || ''
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else ElMessage.error('上传容量配置加载失败')
  }
}

async function saveCapacity() {
  if (capacitySaving.value) return
  capacitySaving.value = true
  try {
    const saved = await updateUploadCapacity({ ...capacityForm })
    capacityUpdatedAt.value = saved?.updated_at || new Date().toISOString()
    ElMessage.success('上传限制已更新，对所有用户立即生效')
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
  } finally { capacitySaving.value = false }
}

async function loadAISettings() {
  try {
    const settings = await getAIAnalysisSettings()
    aiSettingsForm.max_tokens_per_file = Math.min(1000000, Math.max(1, Number(settings?.max_tokens_per_file) || aiSettingsForm.max_tokens_per_file))
    aiSettingsForm.daily_token_quota = Math.max(0, Number(settings?.daily_token_quota) || 0)
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else ElMessage.error('AI 配额加载失败')
  }
}

async function saveAISettings() {
  if (aiSettingsSaving.value) return
  aiSettingsSaving.value = true
  try {
    const payload = {
      max_tokens_per_file: Math.min(1000000, Math.max(1, Number(aiSettingsForm.max_tokens_per_file) || 1)),
      daily_token_quota: Math.max(0, Number(aiSettingsForm.daily_token_quota) || 0)
    }
    const saved = await updateAIAnalysisSettings(payload)
    aiSettingsForm.max_tokens_per_file = Math.min(1000000, Math.max(1, Number(saved?.max_tokens_per_file ?? payload.max_tokens_per_file) || payload.max_tokens_per_file))
    aiSettingsForm.daily_token_quota = Math.max(0, Number(saved?.daily_token_quota ?? payload.daily_token_quota) || 0)
    ElMessage.success('AI 配额已更新')
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else if (error.response?.status === 403) ElMessage.error('仅超级管理员可修改 AI 配额')
    else ElMessage.error(error.response?.data?.message || error.message || 'AI 配额保存失败')
  } finally { aiSettingsSaving.value = false }
}

async function loadKeywordRules() {
  keywordRulesLoading.value = true
  try { keywordRules.value = await getAdminKeywordRules() || [] }
  catch (error) {
    if (error.response?.status === 401) unlocked.value = false
    else ElMessage.error('关键词规则加载失败')
  } finally { keywordRulesLoading.value = false }
}

function selectKeywordFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  if (!/\.(txt|csv)$/i.test(file.name)) { ElMessage.warning('仅支持 TXT 或 CSV 文件'); return }
  if (file.size > 2 * 1024 * 1024) { ElMessage.warning('关键词文件不能超过 2 MB'); return }
  keywordFile.value = file
}

async function submitKeywordImport() {
  if (!keywordFile.value || keywordImporting.value) return
  keywordImporting.value = true
  try {
    const result = await importAdminKeywordRules(keywordFile.value, keywordDefaults)
    ElMessage.success(`导入完成：新增 ${result.created || 0} 条，更新 ${result.updated || 0} 条，跳过 ${result.skipped || 0} 条`)
    keywordFile.value = null
    await loadKeywordRules()
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
  } finally { keywordImporting.value = false }
}

async function removeKeywordRule(rule) {
  try {
    await ElMessageBox.confirm(`确认删除关键词规则“${rule.name}”？删除后将不再参与所有用户的日志解析。`, '删除关键词规则', { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' })
    await deleteAdminKeywordRule(rule.id)
    ElMessage.success('关键词规则已删除')
    await loadKeywordRules()
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  }
}

async function loadActiveModule() {
  const module = modules.value.find(item => item.key === activeModule.value)
  if (module) await selectModule(module)
}

function resetProjectForm() {
  Object.assign(projectForm, {
    name: '',
    product_line: projectOptions.value.lines.find((item) => item.code === 'vehicle')?.code || projectOptions.value.lines[0]?.code || '',
    product_type: projectOptions.value.types.find((item) => item.code === 'dashcam')?.code || projectOptions.value.types[0]?.code || '',
    stage: projectOptions.value.stages.find((item) => item.code === 'production')?.code || projectOptions.value.stages[0]?.code || '',
    description: ''
  })
}
function openCreate() { editingId.value = null; resetProjectForm(); dialogVisible.value = true }
function openEdit(project) { editingId.value = project.id; Object.assign(projectForm, project); dialogVisible.value = true }

async function saveProject() {
  if (!projectForm.name || saving.value) return
  saving.value = true
  try {
    const payload = { name: projectForm.name, product_line: projectForm.product_line, product_type: projectForm.product_type, stage: projectForm.stage, description: projectForm.description }
    if (editingId.value) await updateAdminProject(editingId.value, payload)
    else await createAdminProject(payload)
    dialogVisible.value = false
    ElMessage.success(editingId.value ? '项目已更新' : '项目已创建')
    await loadProjects()
  } catch (error) {
    if (error.response?.status === 401) unlocked.value = false
  } finally { saving.value = false }
}

async function addOption(kind) {
  const name = newOptionNames[kind]?.trim()
  if (!name || optionSaving.value) return
  optionSaving.value = kind
  try {
    await createProjectOption({ kind, name })
    newOptionNames[kind] = ''
    ElMessage.success('选项已添加')
    await loadProjectOptions()
  } finally { optionSaving.value = '' }
}

async function renameOption(item) {
  try {
    const { value } = await ElMessageBox.prompt('请输入新的显示名称', '修改名称', { inputValue: item.name, inputPattern: /^.{1,64}$/, inputErrorMessage: '名称长度为 1 至 64 个字符', confirmButtonText: '保存', cancelButtonText: '取消' })
    await updateProjectOption(item.id, { kind: item.kind, name: value.trim() })
    ElMessage.success('名称已更新')
    await loadProjectOptions()
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  }
}

async function removeOption(item) {
  try {
    await ElMessageBox.confirm(`确认删除“${item.name}”？`, '删除选项', { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' })
    await deleteProjectOption(item.id)
    ElMessage.success('选项已删除')
    await loadProjectOptions()
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  }
}

async function removeProject(project) {
  try {
    await ElMessageBox.confirm(`删除项目 ${project.name} 后，所有用户将无法再选择该项目。`, '删除项目', { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' })
    await deleteAdminProject(project.id)
    ElMessage.success('项目已删除')
    await loadProjects()
  } catch (error) {
    if (error?.response?.status === 401) unlocked.value = false
  }
}

onMounted(async () => {
  unlocked.value = true
  try {
    const user = await getCurrentUser()
    Object.assign(currentUser, user || {})
    Object.assign(access, { role: currentUser.role || 'super_admin', permissions: rolePermissions(currentUser.role || 'super_admin'), open_id: currentUser.feishu_open_id })
    if (currentUser.role === 'user') await enterSelfService(false)
    else {
      activeModule.value = modules.value.find(module => module.ready)?.key || ''
      await loadActiveModule()
    }
  } catch {
    Object.assign(access, { role: 'super_admin', permissions: rolePermissions('super_admin') })
    activeModule.value = 'users'
    await loadUsers().catch(() => {})
    ElMessage.warning('飞书用户信息加载失败，已显示本地管理界面')
  }
  finally {
    checking.value = false
    await nextTick()
  }
})
</script>

<style scoped>
.admin-page{height:100%;overflow:auto;color:#26313e}.checking-state{display:grid;height:100%;place-content:center;justify-items:center;gap:12px;color:#7a8493}.checking-state .el-icon{font-size:30px}.unlock-view{display:grid;max-width:960px;min-height:560px;margin:24px auto;grid-template-columns:1.05fr .95fr;overflow:hidden;border:1px solid #dce2e8;border-radius:8px;background:#fff;box-shadow:0 16px 40px rgba(31,45,61,.08)}.unlock-visual{display:flex;justify-content:center;flex-direction:column;padding:52px;background:#19212c;color:#fff}.shield{display:grid;width:58px;height:58px;place-items:center;margin-bottom:28px;border:1px solid #3c4b5d;border-radius:8px;background:#252f3c;color:#72aafb;font-size:27px}.security-label{color:#78aef8;font-size:11px;font-weight:700}.unlock-visual h1{margin:8px 0 12px;font-size:29px}.unlock-visual p{max-width:340px;margin:0;color:#aeb8c5;font-size:13px;line-height:1.8}.unlock-visual dl{display:grid;gap:13px;margin:42px 0 0}.unlock-visual dl div{display:flex;justify-content:space-between;padding-bottom:12px;border-bottom:1px solid #303b49;font-size:12px}.unlock-visual dt{color:#7f8b9a}.unlock-visual dd{margin:0;color:#d9e0e8}.unlock-form{display:flex;justify-content:center;flex-direction:column;padding:48px}.form-heading{display:flex;align-items:center;gap:13px;margin-bottom:30px}.key-mark{display:grid;width:42px;height:42px;place-items:center;border-radius:7px;background:#edf4ff;color:#2d73d5;font-size:19px}.form-heading h2{margin:0;font-size:20px}.form-heading p{margin:5px 0 0;color:#8a94a3;font-size:12px}.unlock-form :deep(.el-form-item){margin-bottom:20px}.unlock-form .el-alert{margin:-4px 0 18px}.unlock-button{width:100%;margin-top:4px}.page-heading,.section-heading,.toolbar,.row-actions,.section-actions{display:flex;align-items:center}.page-heading{justify-content:space-between;margin-bottom:18px}.page-heading h1{margin:4px 0 0;font-size:23px}.page-heading p,.section-heading p{margin:6px 0 0;color:#7a8493;font-size:12px}.eyebrow{color:#3478dc;font-size:10px;font-weight:700}.module-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px;margin-bottom:16px}.module-card{display:flex;min-width:0;align-items:center;gap:10px;padding:14px;border:1px solid #dde3e9;border-radius:6px;background:#fff;text-align:left;cursor:pointer}.module-card:hover,.module-card.active{border-color:#94b9ec;background:#f7faff}.module-card.disabled{cursor:default}.module-icon{display:grid;width:35px;height:35px;flex:0 0 35px;place-items:center;border-radius:5px;background:#edf4ff;color:#3478dc}.module-copy{display:flex;min-width:0;flex:1;flex-direction:column;gap:4px}.module-copy strong{font-size:12px}.module-copy small{overflow:hidden;color:#8a94a3;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.module-arrow{color:#8a94a3}.project-workspace{padding:18px;border:1px solid #dfe4e9;border-radius:7px;background:#fff}.section-heading{justify-content:space-between}.section-heading h2{margin:0;font-size:16px}.section-actions{gap:8px}.section-actions .el-button{margin:0}.summary-row{display:grid;grid-template-columns:repeat(4,1fr);margin:18px 0;border:1px solid #e4e8ed;border-radius:6px;background:#fafbfc}.summary-row div{display:flex;align-items:center;justify-content:space-between;padding:14px 18px;border-right:1px solid #e4e8ed}.summary-row div:last-child{border:0}.summary-row span{color:#7a8493;font-size:11px}.summary-row strong{font-size:19px}.toolbar{flex-wrap:wrap;gap:10px;margin-bottom:14px}.toolbar .el-input{width:230px}.toolbar .el-select{width:145px}.result-count{margin-left:auto;color:#8a94a3;font-size:11px}.project-name{display:flex;align-items:center;gap:10px}.project-name>span{display:grid;width:35px;height:35px;flex:0 0 35px;place-items:center;border-radius:5px;background:#edf4ff;color:#3478dc;font-size:10px;font-weight:700}.project-name>div{display:flex;min-width:0;flex-direction:column;gap:3px}.project-name strong,.project-name small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.project-name small{color:#8a94a3;font-size:10px}.row-actions{justify-content:center;gap:4px}.row-actions .el-button{margin:0}.mobile-list{display:none}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 14px}.form-grid .el-select{width:100%}.option-manager{display:grid;grid-template-columns:1fr 1fr;gap:16px}.option-group{min-width:0;padding:15px;border:1px solid #e1e6eb;border-radius:6px}.option-heading h3{margin:0;font-size:14px}.option-heading p{margin:5px 0 13px;color:#8a94a3;font-size:10px}.option-create{display:flex;gap:7px}.option-create .el-button{margin:0}.option-list{display:grid;max-height:300px;gap:5px;margin-top:12px;overflow:auto}.option-row{display:flex;min-height:38px;align-items:center;gap:7px;padding:5px 6px 5px 10px;border-radius:4px;background:#f7f9fb}.option-row>span:first-child{min-width:0;flex:1;overflow:hidden;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.option-row>div{display:flex}.option-row .el-button{margin:0}
.settings-workspace{min-height:420px}.resource-settings-grid{display:grid;grid-template-columns:1fr 1fr;gap:28px;margin-top:22px}.resource-setting-section{min-width:0}.ai-quota-section{padding-left:28px;border-left:1px solid rgba(127,151,163,.24)}.resource-setting-heading{display:flex;align-items:baseline;justify-content:space-between;gap:12px}.resource-setting-heading strong{font-size:16px}.resource-setting-heading span{color:#8290a1;font-size:11px}.capacity-overview,.ai-quota-overview{display:grid;grid-template-columns:repeat(3,1fr);margin:20px 0;border:1px solid #e2e7ec;border-radius:6px;background:#f8fafc}.ai-quota-overview{grid-template-columns:1fr}.capacity-overview>div,.ai-quota-overview>div{display:flex;min-width:0;flex-direction:column;gap:8px;padding:18px;border-right:1px solid #e2e7ec}.capacity-overview>div:last-child,.ai-quota-overview>div:last-child{border:0}.capacity-overview span,.ai-quota-overview span{color:#7a8493;font-size:11px}.capacity-overview strong,.ai-quota-overview strong{font-size:20px}.capacity-overview .date-value{font-size:13px;font-weight:600}.capacity-form{max-width:620px}.capacity-form :deep(.el-input-number){width:100%}.field-help{display:block;margin-top:5px;color:#8a94a3;font-size:10px;line-height:1.5}.keyword-layout{display:grid;grid-template-columns:minmax(360px,1fr) minmax(250px,.55fr);gap:18px;margin:20px 0}.keyword-import{padding:16px;border:1px solid #e2e7ec;border-radius:6px;background:#fafbfc}.keyword-import>.el-button{width:100%;margin-top:12px}.keyword-drop{display:flex;width:100%;min-height:78px;align-items:center;gap:13px;padding:14px;border:1px dashed #b9c7d6;border-radius:6px;background:#fff;color:#3976bf;text-align:left;cursor:pointer}.keyword-drop:hover{border-color:#5590dc;background:#f7faff}.keyword-drop>.el-icon{font-size:25px}.keyword-drop>span{display:flex;min-width:0;flex-direction:column;gap:5px}.keyword-drop strong{overflow:hidden;color:#344152;text-overflow:ellipsis;white-space:nowrap}.keyword-drop small{color:#7a8493;font-size:10px}.format-guide{padding:17px;border-left:3px solid #5b91d8;background:#f4f7fa}.format-guide strong{display:block;margin-bottom:11px;font-size:13px}.format-guide code{display:block;overflow:auto;padding:10px;border:1px solid #dde4eb;border-radius:4px;background:#fff;color:#3f5875;font-size:10px;white-space:nowrap}.format-guide p{margin:12px 0 0;color:#677586;font-size:11px;line-height:1.8}.keyword-table{margin-top:4px}
.heading-actions{display:flex;align-items:center;gap:9px}.role-summary{display:grid;grid-template-columns:repeat(4,1fr);margin:18px 0;border:1px solid #e2e7ec;border-radius:6px;background:#fafbfc}.role-summary div{display:flex;align-items:center;justify-content:space-between;padding:14px 18px;border-right:1px solid #e2e7ec}.role-summary div:last-child{border:0}.role-summary span{color:#7a8493;font-size:11px}.role-summary strong{font-size:18px}.user-toolbar .el-input{width:min(340px,100%)}.user-identity{display:flex;min-width:0;align-items:center;gap:10px}.user-identity>span{display:grid;width:34px;height:34px;flex:0 0 34px;place-items:center;border-radius:50%;background:#e9f1fc;color:#2e70c5;font-size:12px;font-weight:700}.user-identity>div{display:flex;min-width:0;flex-direction:column;gap:4px}.user-identity strong,.user-identity small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.user-identity strong{font-size:12px}.user-identity small{color:#8a94a3;font-size:10px}.user-identity em{margin-left:4px;color:#3577ca;font-size:9px;font-style:normal}.role-description{color:#687587;font-size:11px}.settings-workspace :deep(.el-table .el-select){width:128px}
.approval-divider{height:1px;margin:28px 0;background:rgba(127,151,163,.24)}.project-approval-heading{margin-bottom:18px}.project-request-form{margin-bottom:20px;padding:18px;border:1px solid rgba(127,151,163,.22);border-radius:8px;background:rgba(13,22,31,.35)}.project-request-table{margin-top:12px}
.permission-guide{margin:20px 0;padding:18px;border:1px solid rgba(93,207,225,.22);border-radius:8px;background:rgba(11,21,29,.42)}.permission-guide-heading{display:flex;align-items:baseline;justify-content:space-between;gap:14px;margin-bottom:14px}.permission-guide-heading strong{color:#eaf2f5;font-size:14px}.permission-guide-heading span{color:#8fa4af;font-size:11px}.permission-guide-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.permission-guide-item{min-width:0;padding:14px;border:1px solid rgba(215,236,243,.14);border-radius:7px;background:rgba(17,27,35,.54)}.permission-guide-item.current{border-color:rgba(61,208,230,.58);background:rgba(23,112,131,.2)}.permission-guide-item>div{display:flex;align-items:center;justify-content:space-between;gap:8px}.permission-guide-item>div>span{color:#72e3f2;font-size:10px}.permission-guide-item>strong{display:block;margin:12px 0 8px;color:#dce8ed;font-size:12px}.permission-guide-item ul{display:grid;gap:6px;margin:0;padding-left:16px;color:#91a5b2;font-size:10px;line-height:1.55}
.approval-workspace>.section-heading h2{font-size:20px}.approval-workspace>.section-heading p{font-size:13px}.permission-guide-heading strong{font-size:17px}.permission-guide-heading span{font-size:13px}.permission-guide-item>div>span{font-size:12px}.permission-guide-item>strong{font-size:15px}.permission-guide-item ul{font-size:13px;line-height:1.65}.approval-apply-copy strong{font-size:16px}.approval-apply-copy span{font-size:13px}.approval-form :deep(.el-form-item__label){font-size:13px}
.back-approval-button{width:100%;margin:10px 0 0}.approval-workspace{min-height:480px}.approval-apply{display:grid;grid-template-columns:minmax(200px,.65fr) minmax(420px,1.35fr);gap:28px;margin:20px 0 26px;padding:18px;border:1px solid #dce5ef;border-radius:6px;background:#f7f9fc}.approval-apply-copy{display:flex;flex-direction:column;gap:8px;padding:3px 0}.approval-apply-copy strong{font-size:14px}.approval-apply-copy span{color:#748194;font-size:11px;line-height:1.7}.approval-form{display:grid;grid-template-columns:180px minmax(240px,1fr) auto;align-items:start;gap:12px}.approval-form :deep(.el-form-item){margin-bottom:0}.approval-form :deep(.el-select){width:100%}.approval-form-actions{display:flex;align-items:flex-end;height:62px}.approval-list-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin:8px 0 12px}.approval-list-heading>div{display:flex;flex-direction:column;gap:4px}.approval-list-heading strong{font-size:14px}.approval-list-heading span{color:#8290a1;font-size:10px}.approval-applicant{display:flex;min-width:0;flex-direction:column;gap:4px}.approval-applicant strong,.approval-applicant small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.approval-applicant small{color:#8994a3;font-size:10px}.role-change{display:flex;align-items:center;gap:7px}.role-change>.el-icon{color:#9aa5b3;font-size:11px}.reviewer-name{color:#7c8795;font-size:11px}.reviewer-inline{color:#7d8998;font-size:10px}
@media(max-width:1100px){.module-grid{grid-template-columns:repeat(3,1fr)}.summary-row{grid-template-columns:repeat(2,1fr)}.summary-row div:nth-child(2){border-right:0}.summary-row div:nth-child(-n+2){border-bottom:1px solid #e4e8ed}.permission-guide-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:760px){.unlock-view{min-height:auto;margin:0;grid-template-columns:1fr}.unlock-visual{padding:28px}.unlock-visual dl{display:none}.unlock-form{padding:28px}.page-heading{align-items:flex-start}.module-grid{grid-template-columns:1fr 1fr}.module-card{padding:11px}.project-workspace{padding:14px}.section-heading{align-items:flex-start;gap:12px}.approval-workspace>.section-heading{flex-direction:column}.section-actions{align-items:flex-end;flex-direction:column}.toolbar{align-items:stretch;flex-direction:column}.toolbar .el-input,.toolbar .el-select{width:100%}.result-count{margin:0}.desktop-table{display:none}.mobile-list{display:grid;gap:9px}.project-card{padding:13px;border:1px solid #e1e6eb;border-radius:5px}.project-card>div,.project-card footer{display:flex;align-items:center;justify-content:space-between}.project-card p{margin:8px 0 5px;color:#536174;font-size:11px}.project-card>small{color:#8a94a3}.project-card footer{margin-top:11px;padding-top:9px;border-top:1px solid #edf0f3;color:#8a94a3;font-size:10px}.resource-settings-grid{grid-template-columns:1fr}.ai-quota-section{padding-top:26px;padding-left:0;border-top:1px solid rgba(127,151,163,.24);border-left:0}.form-grid,.option-manager,.keyword-layout,.approval-apply,.approval-form,.permission-guide-grid{grid-template-columns:1fr}.approval-form-actions{height:auto}.approval-list-heading,.permission-guide-heading{align-items:flex-start;flex-direction:column}}
@media(max-width:520px){.module-grid{grid-template-columns:1fr}.summary-row,.role-summary{grid-template-columns:1fr}.summary-row div,.role-summary div{border-right:0;border-bottom:1px solid #e4e8ed}.summary-row div:nth-child(2){border-bottom:1px solid #e4e8ed}.role-summary div:last-child{border-bottom:0}.page-heading p{max-width:230px}.heading-actions{align-items:flex-end;flex-direction:column}}

/* Console-wide dark glass theme. */
.admin-page {
  position: relative;
  height: 100%;
  padding: 28px clamp(18px,2.4vw,34px) 44px;
  overflow-x: hidden;
  overflow-y: auto;
  background:
    radial-gradient(circle at 62% -12%,rgba(53,198,221,.11),transparent 40%),
    radial-gradient(circle at 8% 38%,rgba(40,90,148,.08),transparent 35%),
    #080d16;
  color: #e7f0f4;
  scrollbar-color: rgba(91,207,226,.4) transparent;
  scrollbar-width: thin;
}
.admin-page::after { position:absolute;z-index:0;inset:0;pointer-events:none;content:'';background:linear-gradient(114deg,transparent 18%,rgba(93,218,238,.035) 52%,transparent 84%); }
.admin-page > * { position:relative;z-index:1; }
.checking-state { color:#91a6b3; }
.checking-state .el-icon { color:#56d8ea;filter:drop-shadow(0 0 12px rgba(70,210,232,.36)); }
.unlock-view { border-color:rgba(220,238,245,.2);border-radius:16px;background:rgba(27,35,44,.72);box-shadow:inset 0 1px rgba(255,255,255,.09),0 30px 80px rgba(0,0,0,.38); }
.unlock-visual { background:radial-gradient(circle at 20% 12%,rgba(48,183,207,.17),transparent 38%),rgba(10,17,25,.72); }
.shield { border-color:rgba(96,221,239,.28);background:rgba(31,117,136,.22);color:#70e6f4;box-shadow:0 0 28px rgba(50,197,220,.12); }
.security-label,.eyebrow { color:#72e3f2;letter-spacing:.14em;text-transform:uppercase; }
.unlock-visual h1,.form-heading h2,.page-heading h1,.section-heading h2 { color:#f2f7f9; }
.unlock-visual p,.form-heading p,.page-heading p,.section-heading p { color:#91a5b2; }
.unlock-visual dl div { border-bottom-color:rgba(220,238,245,.12); }
.unlock-visual dt { color:#7e929f; }.unlock-visual dd { color:#cfdee4; }
.unlock-form { background:rgba(31,40,49,.54); }
.key-mark { background:rgba(27,133,153,.24);color:#70e6f4; }
.unlock-form :deep(.el-form-item__label),.capacity-form :deep(.el-form-item__label),.keyword-import :deep(.el-form-item__label),.approval-form :deep(.el-form-item__label) { color:#b5c5cd; }
.page-heading { margin-bottom:20px; }.page-heading h1 { font-size:clamp(26px,2.2vw,34px); }
.heading-actions :deep(.el-tag) { border-color:rgba(72,207,228,.3);background:rgba(25,114,132,.22);color:#96ebf4; }
.module-grid { gap:12px;margin-bottom:18px; }
.module-card { border-color:rgba(215,236,243,.16);border-radius:12px;background:rgba(35,45,55,.58);color:#dae7ec;box-shadow:inset 0 1px rgba(255,255,255,.06);transition:transform .24s cubic-bezier(.4,0,.2,1),border-color .24s cubic-bezier(.4,0,.2,1),box-shadow .24s cubic-bezier(.4,0,.2,1); }
.module-card:hover { border-color:rgba(74,210,231,.46);background:rgba(39,58,68,.72);box-shadow:0 14px 30px rgba(0,0,0,.25),0 0 20px rgba(42,189,212,.08);transform:translateY(-2px); }
.module-card.active { border-color:rgba(61,208,230,.65);background:rgba(23,112,131,.28);box-shadow:inset 0 1px rgba(255,255,255,.08),0 0 22px rgba(48,200,223,.12); }
.module-card.disabled { opacity:.52; }.module-icon { background:rgba(26,128,148,.25);color:#72e5f3; }.module-copy strong { color:#eaf2f5; }.module-copy small,.module-arrow { color:#879ca8; }
.project-workspace { padding:20px;border-color:rgba(218,237,244,.2);border-radius:16px;background:rgba(29,38,47,.67);box-shadow:inset 0 1px rgba(255,255,255,.08),0 22px 52px rgba(0,0,0,.24); }
.summary-row,.role-summary,.capacity-overview,.ai-quota-overview { gap:10px;border:0;background:transparent; }
.summary-row div,.role-summary div,.capacity-overview>div,.ai-quota-overview>div { border:1px solid rgba(216,236,243,.14)!important;border-radius:10px;background:rgba(16,25,34,.47);box-shadow:inset 0 1px rgba(255,255,255,.045); }
.summary-row span,.role-summary span,.capacity-overview span,.ai-quota-overview span,.result-count,.field-help { color:#8499a6; }
.summary-row strong,.role-summary strong,.capacity-overview strong,.ai-quota-overview strong { color:#edf4f7; }
.toolbar :deep(.el-input__wrapper),.toolbar :deep(.el-select__wrapper),.capacity-form :deep(.el-input__wrapper),.capacity-form :deep(.el-input-number) { border-color:rgba(214,235,242,.16);background:rgba(7,14,22,.66)!important; }
.project-name>span,.user-identity>span { background:rgba(25,128,148,.24);color:#78e4f1;box-shadow:inset 0 0 0 1px rgba(80,210,230,.2); }
.project-name strong,.user-identity strong { color:#e8f0f4; }.project-name small,.user-identity small,.role-description { color:#8196a3; }
.project-workspace :deep(.el-table) { --el-table-bg-color:transparent;--el-table-tr-bg-color:transparent;--el-table-header-bg-color:rgba(255,255,255,.065);--el-table-row-hover-bg-color:rgba(25,132,153,.18);--el-table-border-color:rgba(216,236,243,.11);--el-table-text-color:#cbd8de;--el-table-header-text-color:#adbdc5;background:transparent;font-family:"JetBrains Mono",Consolas,monospace; }
.project-workspace :deep(.el-table::before) { background:rgba(216,236,243,.13); }
.project-workspace :deep(.el-table th.el-table__cell) { height:43px;background:rgba(255,255,255,.06); }
.project-workspace :deep(.el-table td.el-table__cell) { height:48px;border-bottom-color:rgba(216,236,243,.1);background:transparent; }
.project-workspace :deep(.el-table__row) { animation:admin-row-in .4s cubic-bezier(.4,0,.2,1) both; }
.project-workspace :deep(.el-table__row:nth-child(2)) { animation-delay:.04s; }.project-workspace :deep(.el-table__row:nth-child(3)) { animation-delay:.08s; }.project-workspace :deep(.el-table__row:nth-child(4)) { animation-delay:.12s; }
.project-workspace :deep(.el-table__body tr:hover>td.el-table__cell) { background:rgba(25,132,153,.18)!important; }
.project-workspace :deep(.el-table__body tr:hover>td.el-table__cell:first-child) { box-shadow:inset 2px 0 #35cce1; }
.project-workspace :deep(.el-tag) { border-color:rgba(116,206,219,.26);background:rgba(25,104,121,.2);color:#a7dfe7; }
.project-workspace :deep(.el-button.is-text) { color:#83dce8; }.project-workspace :deep(.el-button--danger.is-text) { color:#ff8290; }
.capacity-form,.keyword-import,.approval-apply { border-color:rgba(214,235,242,.15);border-radius:11px;background:rgba(13,22,31,.47); }
.capacity-form { padding:20px; }.capacity-form :deep(.el-input-number__decrease),.capacity-form :deep(.el-input-number__increase) { border-color:rgba(215,236,243,.12);background:rgba(255,255,255,.05);color:#9ec1cc; }
.keyword-layout { align-items:stretch; }.keyword-import { padding:18px; }
.keyword-drop { border-color:rgba(93,207,225,.34);border-radius:10px;background:rgba(7,14,22,.57);color:#75dfed; }
.keyword-drop:hover { border-color:rgba(83,218,237,.65);background:rgba(25,104,121,.18); }.keyword-drop strong { color:#dce8ed; }.keyword-drop small { color:#8196a2; }
.format-guide { border-left-color:#39c6dc;border-radius:0 10px 10px 0;background:rgba(22,75,88,.22); }.format-guide strong { color:#dce8ed; }.format-guide code { border-color:rgba(215,236,243,.14);background:#0a1119;color:#9ddce6; }.format-guide p { color:#91a5b1; }
.approval-apply { border-color:rgba(77,201,220,.22); }.approval-apply-copy strong,.approval-list-heading strong { color:#eaf2f5; }.approval-apply-copy span,.approval-list-heading span { color:#8fa4af; }
.approval-list-heading :deep(.el-segmented) { --el-segmented-bg-color:rgba(7,14,22,.62);--el-segmented-item-selected-bg-color:rgba(25,128,148,.35);--el-segmented-item-selected-color:#baf3f8;color:#91a5b1; }
.option-group { border-color:rgba(215,236,243,.15);background:rgba(12,21,29,.42); }.option-heading h3 { color:#e8f0f4; }.option-heading p { color:#8499a5; }.option-row { background:rgba(255,255,255,.045);color:#cad8de; }
.project-card { border-color:rgba(215,236,243,.15)!important;background:rgba(17,27,35,.54);color:#dce8ed; }.project-card p,.project-card>small,.project-card footer { color:#859aa6!important; }.project-card footer { border-top-color:rgba(215,236,243,.11)!important; }
@keyframes admin-row-in { from { opacity:0;transform:translateY(8px); } to { opacity:1;transform:translateY(0); } }
@media(max-width:760px) { .admin-page { padding:18px 14px 34px; }.unlock-view { margin:0; }.project-workspace { padding:14px; } }
@media(prefers-reduced-motion:reduce) { .module-card,.project-workspace :deep(.el-table__row) { animation:none;transition:none; } }
.ai-quota-overview { grid-template-columns:repeat(2,1fr); }
</style>
