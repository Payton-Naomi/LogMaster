<template>
    <router-view />
</template>

<script setup>
</script>

<style>
:root {
    --lm-surface-page: #f3f5f7;
    --lm-surface-panel: #ffffff;
    --lm-border: #dfe4ea;
    --lm-text-primary: #202b38;
    --lm-text-secondary: #687586;
    --lm-primary: #2877e8;
    --lm-success: #27936c;
    --lm-warning: #c88720;
    --lm-danger: #cf4f4f;
    font-family: Inter, "Segoe UI", "Microsoft YaHei", Arial, sans-serif;
    color: var(--lm-text-primary);
    font-synthesis: none;
    letter-spacing: 0;
}

*,
*::before,
*::after {
    box-sizing: border-box;
}

html,
body,
#app {
    height: 100%;
    margin: 0;
    padding: 0;
}

body {
    min-width: 320px;
    overflow: hidden;
    background: var(--lm-surface-page);
    color: var(--lm-text-primary);
    -webkit-font-smoothing: antialiased;
}

button,
input,
select,
textarea {
    font: inherit;
    letter-spacing: 0;
}
</style>

<style>
[data-log-theme="light"] .el-select-dropdown,
[data-log-theme="light"] .el-picker-panel,
[data-log-theme="light"] .el-date-picker,
[data-log-theme="light"] .el-time-panel {
  background: rgba(248, 252, 253, .96) !important;
  border-color: rgba(67, 98, 112, .24) !important;
  color: #263d47 !important;
}
[data-log-theme="light"] .el-select-dropdown__item,
[data-log-theme="light"] .el-picker-panel__body,
[data-log-theme="light"] .el-date-table th,
[data-log-theme="light"] .el-date-table td {
  color: #46606c !important;
}
[data-log-theme="light"] .el-select-dropdown__item:hover,
[data-log-theme="light"] .el-select-dropdown__item.selected,
[data-log-theme="light"] .el-date-table td.current:not(.disabled) span,
[data-log-theme="light"] .el-date-table td.available:hover {
  background: rgba(6, 150, 180, .14) !important;
  color: #116e83 !important;
}
[data-log-theme="light"] .detail-drawer.el-drawer,
[data-log-theme="light"] .el-drawer.detail-drawer,
[data-log-theme="light"] .detail-drawer .el-drawer {
  background: rgba(244, 249, 251, .96) !important;
  color: #263d47 !important;
}
[data-log-theme="light"] .detail-drawer .el-drawer__body { background: rgba(244, 249, 251, .96) !important; color: #263d47 !important; }
[data-log-theme="light"] .detail-drawer .detail-pane h2,
[data-log-theme="light"] .detail-drawer .detail-pane dd { color: #17232c !important; }
[data-log-theme="light"] .detail-drawer .detail-pane dt { color: #607782 !important; }
[data-log-theme="dark"] .el-date-table td.in-range .el-date-table-cell {
  background: rgba(6, 182, 212, .16) !important;
}
[data-log-theme="dark"] .el-date-table td.in-range:not(.disabled):hover .el-date-table-cell {
  background: rgba(6, 182, 212, .25) !important;
}
[data-log-theme="dark"] .el-date-table td.start-date .el-date-table-cell__text,
[data-log-theme="dark"] .el-date-table td.end-date .el-date-table-cell__text,
[data-log-theme="dark"] .el-date-table td.current:not(.disabled) .el-date-table-cell__text {
  background: #0b7187 !important;
  color: #ecfeff !important;
  box-shadow: 0 0 0 1px rgba(103, 232, 249, .42), 0 0 14px rgba(6, 182, 212, .2) !important;
}
[data-log-theme="dark"] .el-date-table td.in-range .el-date-table-cell__text {
  color: #d7f7fb !important;
}
</style>

<style>
:root{--lm-surface-page:#0b0d10;--lm-surface-panel:#111418;--lm-border:rgba(255,255,255,.07);--lm-text-primary:#e4e6ea;--lm-text-secondary:#8b9099;--lm-primary:#06b6d4;--lm-success:#34d399;--lm-warning:#f59e0b;--lm-danger:#f43f5e;color-scheme:dark;font-family:Inter,"PingFang SC",system-ui,sans-serif}
body{background:#0b0d10;color:#e4e6ea}
.el-popper,.el-message-box,.el-dialog{background:#111418!important;border-color:rgba(255,255,255,.1)!important;color:#e4e6ea!important}
.el-button{transition:transform .18s ease,box-shadow .18s ease}.el-button:active{transform:scale(.97)}
</style>

<style>
.el-overlay-message-box{background:rgba(3,6,10,.62)!important;}.log-delete-dialog{overflow:hidden!important;width:min(430px,calc(100vw - 32px))!important;padding:0!important;border:1px solid rgba(255,255,255,.18)!important;border-radius:14px!important;background:rgba(24,28,33,.94)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.12),0 28px 80px rgba(0,0,0,.55)!important;font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif!important}.log-delete-dialog .el-message-box__header{padding:20px 22px 10px!important;background:radial-gradient(circle at 15% 0,rgba(244,63,94,.16),transparent 52%)}.log-delete-dialog .el-message-box__title{color:#f5f7fa!important;font-size:17px!important;font-weight:700}.log-delete-dialog .el-message-box__headerbtn{top:15px!important;right:16px!important;width:32px;height:32px;border:1px solid rgba(255,255,255,.12);border-radius:8px;background:rgba(255,255,255,.055)}.log-delete-dialog .el-message-box__headerbtn .el-message-box__close{color:#b7bec8!important}.log-delete-dialog .el-message-box__content{padding:14px 22px 20px!important;color:#c7ced8!important}.log-delete-dialog .el-message-box__status{color:#fb7185!important;filter:drop-shadow(0 0 12px rgba(244,63,94,.38))}.log-delete-dialog .el-message-box__message{line-height:1.7}.log-delete-dialog .el-message-box__btns{gap:9px;padding:14px 22px 20px!important;border-top:1px solid rgba(255,255,255,.1);background:rgba(0,0,0,.13)}.log-delete-dialog .el-button{min-width:84px;border-color:rgba(255,255,255,.16)!important;border-radius:9px!important;background:rgba(255,255,255,.07)!important;color:#d3d9e1!important}.log-delete-dialog .el-button--primary{border-color:rgba(244,63,94,.45)!important;background:rgba(244,63,94,.17)!important;color:#fb7185!important;box-shadow:0 8px 22px rgba(244,63,94,.14)}.log-delete-dialog .el-button:hover{transform:translateY(-1px)}.log-delete-dialog .el-button:active{transform:scale(.95)}
.detail-drawer.el-drawer,.el-drawer.detail-drawer,.detail-drawer .el-drawer{border-left:1px solid rgba(255,255,255,.18)!important;background:rgba(24,28,33,.96)!important;box-shadow:-28px 0 72px rgba(0,0,0,.52)!important}.detail-drawer .el-drawer__body,.el-drawer.detail-drawer .el-drawer__body{padding:0!important;background:rgba(24,28,33,.96)!important;color:#eef2f6!important}.detail-drawer .detail-pane{min-height:100%;padding:26px!important;background:radial-gradient(circle at 50% 0,rgba(103,232,249,.14),transparent 38%)!important;color:#eef2f6!important;font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif!important;-webkit-font-smoothing:antialiased}.detail-drawer .detail-pane h2{color:#f5f7fa!important}.detail-drawer .detail-pane dt{color:#9da6b2!important}.detail-drawer .detail-pane dd{color:#eef2f6!important}.detail-drawer .detail-pane dl{border:1px solid rgba(255,255,255,.12)!important;background:rgba(255,255,255,.055)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.08)}.detail-drawer .detail-top .icon-button{border:1px solid rgba(255,255,255,.14)!important;background:rgba(255,255,255,.07)!important;color:#cbd3dd!important}.detail-drawer .detail-actions{background:linear-gradient(transparent,rgba(24,28,33,.98) 34%)!important}.detail-drawer .primary-button{background:linear-gradient(135deg,#0891b2,#06b6d4)!important;color:#001115!important}.detail-drawer .danger-button{border-color:rgba(244,63,94,.45)!important;background:rgba(244,63,94,.16)!important;color:#fb7185!important}
</style>
<style>
.analysis-context-drawer.el-drawer,.el-drawer.analysis-context-drawer{border-left:1px solid rgba(255,255,255,.18)!important;background:rgba(24,28,33,.96)!important;box-shadow:-28px 0 72px rgba(0,0,0,.52)!important;color:#eef2f6!important}.analysis-context-drawer .el-drawer__header{margin:0!important;padding:20px 24px 14px!important;border-bottom:1px solid rgba(255,255,255,.1);background:radial-gradient(circle at 12% 0,rgba(103,232,249,.13),transparent 54%);color:#f5f7fa!important}.analysis-context-drawer .el-drawer__title{color:#f5f7fa!important;font-size:16px!important;font-weight:700}.analysis-context-drawer .el-drawer__close-btn{color:#aeb7c3!important}.analysis-context-drawer .el-drawer__body{padding:20px 24px!important;background:rgba(24,28,33,.96)!important;color:#eef2f6!important}.analysis-context-drawer .detail-head{padding-bottom:15px;border-bottom:1px solid rgba(255,255,255,.12)}.analysis-context-drawer .detail-head strong,.analysis-context-drawer .section-title h3,.analysis-context-drawer .detail-meta dd{color:#f5f7fa!important}.analysis-context-drawer .detail-head>span,.analysis-context-drawer .detail-meta dt,.analysis-context-drawer .section-title span{color:#9da6b2!important}.analysis-context-drawer .detail-meta{gap:10px 12px;margin:16px 0 22px!important}.analysis-context-drawer .detail-meta div{padding:10px 12px;border:1px solid rgba(255,255,255,.1);border-radius:9px;background:rgba(255,255,255,.045)}.analysis-context-drawer .cause-row{padding:11px 0;border-bottom-color:rgba(255,255,255,.1)}.analysis-context-drawer .cause-row p,.analysis-context-drawer .cause-row code{color:#c7ced8!important}.analysis-context-drawer .context-window{border:1px solid rgba(255,255,255,.14)!important;border-radius:10px!important;background:#0d1117!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.06)}.analysis-context-drawer .context-line{color:#dce3eb!important}.analysis-context-drawer .context-line.hit{background:rgba(244,63,94,.2)!important;color:#fff!important}.analysis-context-drawer .context-line.cause{background:rgba(245,158,11,.13)!important}.analysis-context-drawer .line-number{color:#7f91a7!important}.analysis-context-drawer .line-time{color:#8db4d8!important}.analysis-context-drawer .fallback-line{color:#eef2f6!important}
</style>

<style>
/* Keep teleported Element Plus controls consistent with the dark glass surfaces. */
.el-select__wrapper,
.el-input__wrapper,
.el-textarea__inner { background: rgba(8, 15, 23, .72) !important; color: #e5f0f4 !important; }
.el-select__selected-item,
.el-select__placeholder,
.el-input__inner,
.el-textarea__inner { color: #dcebf0 !important; }
.el-select__placeholder,
.el-input__inner::placeholder,
.el-textarea__inner::placeholder { color: #78909d !important; }
.el-select__wrapper .el-tag,
.el-select .el-tag,
.el-select-dropdown .el-tag {
    border-color: rgba(83, 205, 224, .3) !important;
    background: rgba(26, 91, 106, .34) !important;
    color: #bcecf3 !important;
}
.el-input__count,
.el-input__count-inner { background: rgba(18, 29, 39, .94) !important; color: #8198a7 !important; }
.el-select-dropdown,
.el-picker-panel,
.el-date-picker,
.el-time-panel { border-color: rgba(220, 238, 245, .18) !important; background: rgba(17, 24, 32, .98) !important; color: #dcebf0 !important; }
.el-select-dropdown__item { color: #a9bbc5 !important; }
.el-select-dropdown__item.hover,
.el-select-dropdown__item:hover,
.el-select-dropdown__item.selected { background: rgba(26, 143, 163, .22) !important; color: #c7f5fb !important; }
.rule-dialog.el-dialog {
    overflow: hidden;
    width: min(560px, calc(100vw - 28px)) !important;
    border: 1px solid rgba(220, 238, 245, .2) !important;
    border-radius: 15px !important;
    background: rgba(25, 32, 40, .96) !important;
    box-shadow: inset 0 1px rgba(255,255,255,.1), 0 30px 90px rgba(0,0,0,.6) !important;
    
}
.rule-dialog .el-dialog__header { margin: 0; padding: 20px 22px 16px; border-bottom: 1px solid rgba(220,238,245,.12); background: radial-gradient(circle at 12% 0, rgba(73,207,229,.13), transparent 58%); }
.rule-dialog .el-dialog__title { color: #f2f7f9; font-size: 17px; font-weight: 700; }
.rule-dialog .el-dialog__headerbtn .el-dialog__close { color: #9eb1bc; }
.rule-dialog .el-dialog__body { padding: 20px 22px; color: #dce8ed; }
.rule-dialog .el-dialog__footer { padding: 14px 22px 20px; border-top: 1px solid rgba(220,238,245,.1); }
.rule-dialog .el-form-item__label { color: #aebfc7 !important; }
.rule-dialog .el-checkbox__label { color: #c8d6dc !important; }
.rule-dialog .el-input__wrapper,
.rule-dialog .el-select__wrapper,
.rule-dialog .el-textarea__inner { border: 1px solid rgba(220,238,245,.18); background: rgba(7,14,22,.7)!important; box-shadow: inset 0 1px rgba(255,255,255,.04); }
.rule-dialog .el-input__wrapper:hover,
.rule-dialog .el-select__wrapper:hover,
.rule-dialog .el-textarea__inner:focus { border-color: rgba(75,211,233,.58); }
.admin-glass-dialog.el-dialog {
    overflow: hidden;
    border: 1px solid rgba(220,238,245,.2) !important;
    border-radius: 15px !important;
    background: rgba(25,32,40,.97) !important;
    color: #dce8ed !important;
    box-shadow: inset 0 1px rgba(255,255,255,.1),0 30px 90px rgba(0,0,0,.62) !important;
    
}
.admin-glass-dialog .el-dialog__header { margin:0;padding:20px 22px 16px;border-bottom:1px solid rgba(220,238,245,.12);background:radial-gradient(circle at 12% 0,rgba(72,206,228,.13),transparent 58%); }
.admin-glass-dialog .el-dialog__title { color:#f2f7f9;font-size:17px;font-weight:700; }
.admin-glass-dialog .el-dialog__headerbtn .el-dialog__close { color:#9eb1bc; }
.admin-glass-dialog .el-dialog__body { padding:20px 22px;color:#dce8ed; }
.admin-glass-dialog .el-dialog__footer { padding:14px 22px 20px;border-top:1px solid rgba(220,238,245,.1); }
.admin-glass-dialog .el-form-item__label { color:#afc0c8!important; }
.admin-glass-dialog .el-input__wrapper,
.admin-glass-dialog .el-select__wrapper,
.admin-glass-dialog .el-textarea__inner { border:1px solid rgba(220,238,245,.17);background:rgba(7,14,22,.72)!important;box-shadow:inset 0 1px rgba(255,255,255,.04); }
.admin-glass-dialog .option-group { border-color:rgba(215,236,243,.15);background:rgba(10,18,26,.48); }
.admin-glass-dialog .option-heading h3 { color:#e8f0f4; }
.admin-glass-dialog .option-heading p { color:#8297a3; }
.admin-glass-dialog .option-row { background:rgba(255,255,255,.045);color:#cad8de; }
</style>

<style>
[data-log-theme="light"] .el-overlay { background: rgba(33, 55, 64, .28) !important; }
[data-log-theme="light"] .rule-dialog.el-dialog,
[data-log-theme="light"] .admin-glass-dialog.el-dialog,
[data-log-theme="light"] .log-delete-dialog.el-message-box,
[data-log-theme="light"] .analysis-context-drawer.el-drawer {
  border-color: rgba(67, 98, 112, .25) !important;
  background: rgba(245, 250, 251, .97) !important;
  color: #17303b !important;
  box-shadow: inset 0 1px rgba(255,255,255,.9), 0 26px 72px rgba(35,67,83,.24) !important;
}
[data-log-theme="light"] .rule-dialog .el-dialog__header,
[data-log-theme="light"] .admin-glass-dialog .el-dialog__header,
[data-log-theme="light"] .log-delete-dialog .el-message-box__header,
[data-log-theme="light"] .analysis-context-drawer .el-drawer__header {
  border-color: rgba(67, 98, 112, .18) !important;
  background: radial-gradient(circle at 12% 0, rgba(6,150,180,.12), transparent 58%) !important;
}
[data-log-theme="light"] .rule-dialog .el-dialog__title,
[data-log-theme="light"] .admin-glass-dialog .el-dialog__title,
[data-log-theme="light"] .log-delete-dialog .el-message-box__title,
[data-log-theme="light"] .analysis-context-drawer .el-drawer__title,
[data-log-theme="light"] .analysis-context-drawer h3 { color: #17303b !important; }
[data-log-theme="light"] .rule-dialog .el-dialog__body,
[data-log-theme="light"] .admin-glass-dialog .el-dialog__body,
[data-log-theme="light"] .analysis-context-drawer .el-drawer__body { background: rgba(245,250,251,.97) !important; color: #263d47 !important; }
[data-log-theme="light"] .rule-dialog .el-form-item__label,
[data-log-theme="light"] .admin-glass-dialog .el-form-item__label,
[data-log-theme="light"] .rule-dialog .el-checkbox__label { color: #46606c !important; }
[data-log-theme="light"] .rule-dialog .el-input__wrapper,
[data-log-theme="light"] .rule-dialog .el-select__wrapper,
[data-log-theme="light"] .rule-dialog .el-textarea__inner,
[data-log-theme="light"] .admin-glass-dialog .el-input__wrapper,
[data-log-theme="light"] .admin-glass-dialog .el-select__wrapper,
[data-log-theme="light"] .admin-glass-dialog .el-textarea__inner {
  border-color: rgba(67,98,112,.24) !important;
  background: rgba(255,255,255,.7) !important;
  color: #17303b !important;
  box-shadow: inset 0 1px rgba(255,255,255,.86) !important;
}
[data-log-theme="light"] .rule-dialog input,
[data-log-theme="light"] .rule-dialog textarea,
[data-log-theme="light"] .admin-glass-dialog input,
[data-log-theme="light"] .admin-glass-dialog textarea { color: #17303b !important; }
[data-log-theme="light"] .admin-glass-dialog .option-group,
[data-log-theme="light"] .admin-glass-dialog .option-row,
[data-log-theme="light"] .analysis-context-drawer .detail-meta div {
  border-color: rgba(67,98,112,.2) !important;
  background: rgba(225,238,242,.58) !important;
  color: #263d47 !important;
}

/* Browser zoom reduces the CSS viewport; keep data grids scrollable instead of overlapping columns. */
.el-table { max-width: 100%; }
.el-table .cell { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
@media (max-width: 1180px) {
    .el-table__fixed,
    .el-table__fixed-right { box-shadow: none !important; }
}
[data-log-theme="light"] .detail-drawer.el-drawer,
[data-log-theme="light"] .el-drawer.detail-drawer {
  border-color: rgba(67, 98, 112, .28) !important;
  background: #f4fafb !important;
  color: #17303b !important;
  box-shadow: -24px 0 64px rgba(35, 67, 83, .28) !important;
}
[data-log-theme="light"] .detail-drawer .el-drawer__body,
[data-log-theme="light"] .el-drawer.detail-drawer .el-drawer__body {
  background: #f4fafb !important;
  color: #17303b !important;
}
[data-log-theme="light"] .detail-drawer .detail-pane {
  background: radial-gradient(circle at 50% 0, rgba(6, 150, 180, .13), transparent 38%) !important;
  color: #17303b !important;
}
[data-log-theme="light"] .detail-drawer .detail-pane h2,
[data-log-theme="light"] .detail-drawer .detail-pane dd { color: #17303b !important; }
[data-log-theme="light"] .detail-drawer .detail-pane dt { color: #58707c !important; }
[data-log-theme="light"] .detail-drawer .detail-pane .kicker { color: #087990 !important; }
[data-log-theme="light"] .detail-drawer .detail-pane .level-chip {
  border-color: rgba(67, 98, 112, .28) !important;
  background: rgba(255, 255, 255, .78) !important;
  color: #365461 !important;
}
[data-log-theme="light"] .detail-drawer .detail-pane dl {
  border-color: rgba(67, 98, 112, .24) !important;
  background: rgba(255, 255, 255, .72) !important;
  box-shadow: inset 0 1px rgba(255, 255, 255, .92), 0 8px 24px rgba(35, 67, 83, .08) !important;
}
[data-log-theme="light"] .detail-drawer .detail-top .icon-button {
  border-color: rgba(67, 98, 112, .28) !important;
  background: rgba(255, 255, 255, .78) !important;
  color: #365461 !important;
}
[data-log-theme="light"] .detail-drawer .detail-actions { background: linear-gradient(transparent, #f4fafb 34%) !important; }
[data-log-theme="light"] .log-delete-dialog .el-message-box__content,
[data-log-theme="light"] .log-delete-dialog .el-message-box__message { color: #263d47 !important; }
[data-log-theme="light"] .log-delete-dialog .el-message-box__status { color: #e11d48 !important; }
[data-log-theme="light"] .log-delete-dialog .el-message-box__headerbtn {
  border-color: rgba(67, 98, 112, .22) !important;
  background: rgba(255, 255, 255, .72) !important;
}
[data-log-theme="light"] .log-delete-dialog .el-message-box__headerbtn .el-message-box__close { color: #526b76 !important; }
[data-log-theme="light"] .log-delete-dialog .el-message-box__btns {
  border-color: rgba(67, 98, 112, .18) !important;
  background: rgba(224, 237, 241, .62) !important;
}
[data-log-theme="light"] .log-delete-dialog .el-button {
  border-color: rgba(67, 98, 112, .3) !important;
  background: #fff !important;
  color: #365461 !important;
}
[data-log-theme="light"] .log-delete-dialog .el-button--primary {
  border-color: #e11d48 !important;
  background: #e11d48 !important;
  color: #fff !important;
  box-shadow: 0 8px 20px rgba(225, 29, 72, .2) !important;
}
</style>

<style>
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) {
  background:
    radial-gradient(ellipse at 12% 8%, rgba(103, 232, 249, .22), transparent 30%),
    radial-gradient(ellipse at 86% 22%, rgba(129, 140, 248, .15), transparent 34%),
    radial-gradient(ellipse at 50% 100%, rgba(14, 116, 144, .12), transparent 48%),
    linear-gradient(135deg, #b8cbd5 0%, #d9e6eb 38%, #c3d6df 66%, #a9bfca 100%) !important;
  color: #17232c !important;
}

html[data-log-theme="light"] body #app .particle-backdrop {
  background:
    radial-gradient(circle at 52% 10%, rgba(255,255,255,.42), transparent 35%),
    radial-gradient(circle at 70% 72%, rgba(56,189,248,.1), transparent 48%),
    rgba(218, 234, 239, .12) !important;
}
html[data-log-theme="light"] body #app .particle-backdrop canvas {
  opacity: .52 !important;
  mix-blend-mode: multiply;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .page-heading,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .page-header,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page).page > header,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary-item,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .panel,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .list-panel,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .detail-panel,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .editor-panel,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .project-workspace,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .result-panel,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page).rule-config > .summary > div {
  border-color: rgba(255, 255, 255, .58) !important;
  background: linear-gradient(145deg, rgba(255,255,255,.48), rgba(218,237,242,.24)) !important;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.88), inset 0 -1px rgba(67,98,112,.12), 0 18px 48px rgba(32,70,86,.18) !important;
  color: #17232c !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) h1,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) h2,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) h3,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary-item strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .panel-heading strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .card-title-row h2,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .detail-heading h2,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .editor-heading h2,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .meta dd,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .file-info strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .project-name strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .user-identity strong {
  color: #17232c !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) p,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) small,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) dt,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary-item span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .status-control,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .detail-status,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .detail-switch,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .block-label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .result-count,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .field-help,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) footer {
  color: #55727d !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .analysis-tip {
  border-color: rgba(6, 150, 180, .24) !important;
  background: rgba(191, 225, 233, .58) !important;
  color: #315866 !important;
  box-shadow: inset 0 1px rgba(255,255,255,.66), 0 12px 32px rgba(35,67,83,.1) !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .analysis-tip .el-alert__title,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .analysis-tip .el-alert__description {
  color: #315866 !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .scenario-card,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .info-block,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .enable-setting,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .module-card,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary-row > div,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .role-summary > div,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .capacity-overview > div,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .approval-apply,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .format-guide,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-view,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-visual,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-form,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .meta dl > div,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .counts > div > span {
  border-color: rgba(67, 98, 112, .2) !important;
  background: rgba(255,255,255,.38) !important;
  box-shadow: inset 0 1px rgba(255,255,255,.68) !important;
  color: #263d47 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-visual::before {
  opacity: .2 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .security-label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .form-heading p {
  color: #55727d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .scenario-card:hover,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .module-card:hover {
  border-color: rgba(6, 150, 180, .42) !important;
  background: rgba(255,255,255,.55) !important;
  box-shadow: inset 0 1px rgba(255,255,255,.78), 0 16px 34px rgba(32,70,86,.17), 0 0 22px rgba(6,182,212,.12) !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .drop-zone,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .analysis-mode,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .keyword-drop {
  border-color: rgba(47, 116, 137, .32) !important;
  background: rgba(231, 243, 246, .62) !important;
  color: #315866 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .analysis-mode.exclusive {
  border-color: rgba(180, 120, 26, .32) !important;
  background: rgba(255, 244, 218, .64) !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .drop-zone strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .analysis-mode-heading strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .waiting-state strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .task-state dd {
  color: #263d47 !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-input__wrapper,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-select__wrapper,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-textarea__inner {
  border-color: rgba(67,98,112,.23) !important;
  background: rgba(255,255,255,.6) !important;
  color: #17303b !important;
  box-shadow: 0 0 0 1px rgba(67,98,112,.18) inset, inset 0 1px rgba(255,255,255,.8) !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) input,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) textarea,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-select__selected-item,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-form-item__label {
  color: #263d47 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) input::placeholder,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) textarea::placeholder {
  color: #718692 !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table {
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-header-bg-color: rgba(44,78,92,.1);
  --el-table-row-hover-bg-color: rgba(6,150,180,.11);
  --el-table-border-color: rgba(49,87,101,.17);
  --el-table-text-color: #263d47;
  --el-table-header-text-color: #55727d;
  background: transparent !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table th.el-table__cell {
  border-color: rgba(49,87,101,.18) !important;
  background: rgba(44,78,92,.1) !important;
  color: #55727d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table td.el-table__cell {
  border-color: rgba(49,87,101,.13) !important;
  background: rgba(255,255,255,.06) !important;
  color: #263d47 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table__row:hover td.el-table__cell {
  background: rgba(6,150,180,.12) !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-button--default {
  border-color: rgba(47,92,110,.25) !important;
  background: rgba(255,255,255,.46) !important;
  color: #315866 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-radio-button__inner {
  border-color: rgba(47,92,110,.22) !important;
  background: rgba(255,255,255,.38) !important;
  color: #55727d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-radio-button__original-radio:checked + .el-radio-button__inner {
  border-color: rgba(6,150,180,.48) !important;
  background: rgba(6,150,180,.17) !important;
  color: #0b667b !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .danger,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .error { color: #d9435c !important; }
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .warning { color: #b97809 !important; }
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .file-ready,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .scenario-note { color: #16805d !important; }
</style>

<style>
/* Day mode typography uses dark ink even for secondary and disabled information. */
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) {
  --lm-day-text: #1f3540;
  --lm-day-secondary: #46616d;
  --lm-day-muted: #5b727d;
  --lm-day-accent: #087f98;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) p,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) small,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) dt,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) dd {
  color: var(--lm-day-secondary) !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) h1,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) h2,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) h3 {
  color: var(--lm-day-text) !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-icon,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-button > span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-checkbox__input + .el-checkbox__label {
  color: inherit !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .eyebrow,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .kicker,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .security-label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .editor-heading > div > span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .block-heading em,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .field-meta strong,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .card-footer > span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .copy-task {
  color: var(--lm-day-accent) !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .summary-row span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .role-summary span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .capacity-overview span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .module-copy small,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .project-name small,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .user-identity small,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .role-description,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .result-count,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .field-help,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .approval-apply-copy span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .approval-list-heading span,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .reviewer-name,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .reviewer-inline,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .muted,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .empty-text,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .file-empty,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .submit-hint {
  color: var(--lm-day-muted) !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-visual dt {
  color: #46616d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-visual dd,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .unlock-form .el-form-item__label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .capacity-form .el-form-item__label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .keyword-import .el-form-item__label,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .approval-form .el-form-item__label {
  color: var(--lm-day-text) !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) input::placeholder,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) textarea::placeholder,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-select__placeholder {
  color: #607782 !important;
  opacity: 1 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-input__prefix,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-input__suffix,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-select__caret {
  color: #526b76 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-input__count,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-input__count-inner {
  background: transparent !important;
  color: #526b76 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table th.el-table__cell,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table th.el-table__cell .cell {
  color: #46616d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table td.el-table__cell,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-table td.el-table__cell .cell {
  color: #263d47 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-segmented {
  --el-segmented-bg-color: rgba(54,84,96,.1) !important;
  --el-segmented-color: #46616d !important;
  --el-segmented-item-selected-bg-color: rgba(6,150,180,.17) !important;
  --el-segmented-item-selected-color: #075a6d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-segmented__item-selected span {
  color: #075a6d !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .level-filter > span {
  color: #46616d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .level-filter button {
  border-color: rgba(58, 91, 104, .28) !important;
  background: rgba(255,255,255,.42) !important;
  color: #3f5965 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .level-filter button:hover {
  background: rgba(6,150,180,.12) !important;
  color: #0b6479 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .level-filter button.active {
  border-color: rgba(6,150,180,.48) !important;
  background: rgba(6,150,180,.18) !important;
  color: #075a6d !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .level-filter em,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .level-filter button.active em {
  background: rgba(54,84,96,.1) !important;
  color: #526b76 !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag {
  border-color: rgba(67,98,112,.24) !important;
  background: rgba(255,255,255,.42) !important;
  color: #46616d !important;
  opacity: 1 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag span {
  color: inherit !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag--danger {
  border-color: rgba(217,67,92,.38) !important;
  background: rgba(217,67,92,.1) !important;
  color: #c72f4a !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag--warning {
  border-color: rgba(185,120,9,.38) !important;
  background: rgba(185,120,9,.1) !important;
  color: #976000 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag--primary,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag--info {
  border-color: rgba(8,127,152,.3) !important;
  background: rgba(8,127,152,.09) !important;
  color: #0a6b80 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-tag--success {
  border-color: rgba(22,128,93,.32) !important;
  background: rgba(22,128,93,.1) !important;
  color: #126c4f !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-button.is-link,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-button.is-text {
  color: var(--lm-day-accent) !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-button.is-disabled,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .el-button.is-disabled:hover {
  border-color: rgba(67,98,112,.2) !important;
  background: rgba(116,169,184,.26) !important;
  color: #46616d !important;
  opacity: 1 !important;
}

html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .error,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .danger,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) span.error,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) span.danger,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) strong.error,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) strong.danger {
  color: #c72f4a !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .warning,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) span.warning,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) strong.warning {
  color: #976000 !important;
}
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .file-ready,
html[data-log-theme="light"] body #app .shared-particle-surface:not(.tasks-page) .scenario-note {
  color: #126c4f !important;
}
</style>
