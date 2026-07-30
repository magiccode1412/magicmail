<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
  向导 Step3: 高级设置（可选，默认折叠面板）
  包含：同步设置、SMTP发信、HTTP代理
-->
<template>
  <div class="advanced-step">
    <div class="step-title-bar">
      <ChevronLeft :size="18" class="back-btn" @click="$emit('back')" />
      <h4 class="step-title">高级设置</h4>
      <span class="optional-hint">可选</span>
    </div>

    <p class="step-desc">以下均为可选配置，可以直接完成添加，后续也可以在编辑中修改</p>

    <!-- 同步设置 -->
    <details class="adv-section" open>
      <summary class="adv-summary">
        <Mail :size="15" class="section-icon" />
        同步设置
        <span class="toggle-arrow">&#x25B6;</span>
      </summary>
      <div class="adv-body">
        <div class="form-group">
          <label class="form-label">同步范围</label>
          <div class="radio-group">
            <label v-for="opt in syncModes" :key="opt.value"
              class="radio-option" :class="{ active: form.sync_mode === opt.value }">
              <input type="radio" :value="opt.value" v-model="form.sync_mode" hidden />
              <span>{{ opt.label }}</span>
              <small>{{ opt.desc }}</small>
            </label>
          </div>
        </div>
        <div class="form-group" v-if="form.sync_mode === 'recent'">
          <label class="form-label">最近天数</label>
          <div class="days-input-row">
            <input v-model.number="form.sync_days" type="number" class="input days-input" min="1" max="365" />
            <span class="unit">天</span>
          </div>
        </div>
        <div class="form-group toggle-row">
          <label class="form-label">删除时同步删除源服务器邮件</label>
          <label class="toggle-switch">
            <input type="checkbox" v-model="form.delete_on_server" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>
    </details>

    <!-- SMTP 发信 -->
    <details class="adv-section">
      <summary class="adv-summary">
        <Send :size="15" class="section-icon" />
        SMTP 发信服务器（可选）
        <span class="toggle-arrow">&#x25B6;</span>
      </summary>
      <div class="adv-body">
        <p class="hint-text">用于发送邮件，不填则使用收信服务器地址</p>
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">SMTP 服务器</label>
            <input v-model="form.smtp_host" type="text" class="input" placeholder="smtp.example.com" />
          </div>
          <div class="form-group port-group">
            <label class="form-label">端口</label>
            <input v-model.number="form.smtp_port" type="number" class="input" min="1" max="65535" placeholder="587" />
          </div>
        </div>
      </div>
    </details>

    <!-- HTTP 代理 -->
    <details class="adv-section">
      <summary class="adv-summary">
        <Globe :size="15" class="section-icon" />
        HTTP 代理（可选）
        <span class="toggle-arrow">&#x25B6;</span>
      </summary>
      <div class="adv-body">
        <div class="form-group toggle-row">
          <label class="form-label">启用代理</label>
          <label class="toggle-switch">
            <input type="checkbox" v-model="form.proxy_enabled" />
            <span class="toggle-slider"></span>
          </label>
        </div>
        <div class="form-group" v-if="form.proxy_enabled">
          <label class="form-label">代理地址</label>
          <input v-model="form.proxy_url" type="text" class="input" placeholder="http://user:pass@host:port" />
          <p class="hint-text">支持 HTTP/SOCKS5 代理</p>
        </div>
      </div>
    </details>

    <!-- 底部操作 -->
    <div class="form-actions">
      <button type="button" class="btn btn-secondary" @click="$emit('back')">返回上一步</button>
      <button type="button" class="btn btn-ghost" @click="$emit('complete')">跳过</button>
      <button type="button" class="btn btn-primary" @click="$emit('complete')">完成 ✓</button>
    </div>
  </div>
</template>

<script setup>
import { Mail, Send, Globe, ChevronLeft } from 'lucide-vue-next'

const props = defineProps({
  formData: { type: Object, required: true }
})

defineEmits(['next', 'back', 'complete'])

const form = props.formData

const syncModes = [
  { value: 'unread', label: '只同步未读', desc: '默认，速度快' },
  { value: 'all',    label: '全部邮件',   desc: '含已读邮件' },
  { value: 'recent', label: '最近N天',     desc: '限定时间范围' }
]
</script>

<style scoped>
.advanced-step {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.step-title-bar {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.back-btn {
  cursor: pointer; color: var(--text-tertiary);
  padding: 4px; border-radius: var(--radius-sm);
}
.back-btn:hover { color: var(--primary-500); background: var(--bg-hover); }
.step-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
}
.optional-hint {
  font-size: 11px;
  color: var(--text-tertiary);
  background: var(--bg-tertiary);
  padding: 2px 8px;
  border-radius: var(--radius-full);
  margin-left: auto;
}

.step-desc {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  margin: 0;
}

/* ---- 折叠面板 ---- */
.adv-section {
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.adv-section summary { list-style: none; }
.adv-section summary::-webkit-details-marker { display: none; }

.adv-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 14px;
  background: var(--bg-secondary);
  cursor: pointer;
  user-select: none;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  transition: background 0.2s;
}
.adv-summary:hover { background: var(--bg-hover); }
.section-icon { color: var(--primary-500); flex-shrink: 0; }

.toggle-arrow {
  margin-left: auto;
  font-size: 10px;
  color: var(--text-tertiary);
  transition: transform 0.2s;
}
.adv-section[open] .toggle-arrow { transform: rotate(90deg); }

.adv-body {
  padding: 14px 16px 12px;
  border-top: 1px solid var(--border-light);
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.hint-text { font-size: var(--font-size-xs); color: var(--text-tertiary); margin-top: 2px; }

/* ---- 表单元素 ---- */
.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
}
.input {
  padding: 9px 13px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-family: inherit;
  outline: none;
  transition: border-color 0.2s;
}
.input:focus { border-color: var(--primary-400); box-shadow: 0 0 0 3px var(--mail-unread-bg, var(--primary-100)); }
.input::placeholder { color: var(--text-tertiary); opacity: 0.6; }

.form-row { display: flex; gap: var(--space-md); }
.flex-1 { flex: 1; }
.port-group { width: 100px; }

/* Radio 选项 */
.radio-group { display: flex; flex-direction: column; gap: 6px; }
.radio-option {
  display: flex; align-items: center; gap: 10px;
  padding: 9px 13px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  user-select: none;
  transition: all 0.2s;
}
.radio-option:hover { background: var(--bg-hover); border-color: var(--primary-300); }
.radio-option.active { border-color: var(--primary-500); background: var(--selected-bg, var(--primary-50)); }
.radio-option span { font-size: var(--font-size-sm); font-weight: var(--font-weight-medium); color: var(--text-primary); }
.radio-option small { font-size: 11px; color: var(--text-tertiary); margin-left: auto; }

.days-input-row { display: flex; align-items: center; gap: 8px; }
.days-input { width: 100px; }
.unit { font-size: var(--font-size-sm); color: var(--text-secondary); }

/* Toggle 开关 */
.toggle-row {
  flex-direction: row !important;
  align-items: center;
  justify-content: space-between;
}
.toggle-switch {
  position: relative;
  display: inline-block;
  width: 44px; height: 24px; cursor: pointer;
}
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider {
  position: absolute; inset: 0;
  background: var(--border-color); border-radius: 12px;
  transition: background 0.2s;
}
.toggle-slider::after {
  content: ''; position: absolute;
  width: 18px; height: 18px;
  left: 3px; bottom: 3px;
  background: #fff; border-radius: 50%;
  transition: transform 0.2s;
}
.toggle-switch input:checked + .toggle-slider { background: var(--primary-500); }
.toggle-switch input:checked + .toggle-slider::after { transform: translateX(20px); }

/* ---- 底部操作栏（复用全局主题自适应样式 .btn / .btn-primary / .btn-secondary / .btn-ghost）---- */
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  padding-top: var(--space-md);
  border-top: 1px solid var(--border-light);
  margin-top: var(--space-sm);
}

@media (max-width: 480px) {
  .form-row { flex-direction: column; }
  .port-group { width: unset; }
  .form-actions { flex-wrap: wrap; }
}
</style>
