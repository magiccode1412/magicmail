<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="account-manage">
    <!-- 页面标题栏 -->
    <div class="page-header">
      <h2 class="page-title">邮箱账号管理</h2>
      <button class="btn btn-primary" @click="openForm()">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
        添加账号
      </button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading && accounts.length === 0" class="loading-state">
      <div class="spinner"></div>
    </div>

    <!-- 账号列表 -->
    <div v-if="accounts.length > 0" class="account-list">
      <div
        v-for="acc in accounts"
        :key="acc.id"
        class="account-item"
        :class="{ 'is-disabled': acc.status === 'disabled' }"
      >
        <!-- 左侧：图标 + 基本信息 -->
        <div class="item-left">
          <div class="item-icon" :class="{ 'has-brand': getProviderIcon(acc) }">
            <!-- 服务商品牌图标 -->
            <span v-if="getProviderIcon(acc)" class="icon-svg" v-html="getProviderIcon(acc)"></span>
            <!-- 默认信封图标 -->
            <svg v-else width="20" height="20" viewBox="0 0 22 22" fill="none">
              <rect x="2.5" y="5.5" width="17" height="12" rx="2" stroke="currentColor" stroke-width="1.6"/>
              <path d="M2.5 7L11 13L19.5 7" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
            </svg>
          </div>
          <div class="item-info">
            <strong class="item-name">{{ acc.name }}</strong>
            <span class="item-email">{{ acc.email }}</span>
          </div>
          <span
            class="status-dot"
            :class="acc.status"
            :title="acc.status === 'active' ? '正常' : (acc.error_msg || '异常')"
          ></span>
        </div>

        <!-- 中间：详情 -->
        <div class="item-details">
          <span class="detail-tag">{{ (acc.protocol || 'imap').toUpperCase() }} {{ acc.host }}:{{ acc.port }}</span>
          <span class="detail-tag">邮件 {{ acc.mail_count }}</span>
          <span class="detail-tag text-primary">未读 {{ acc.unread_count }}</span>
          <span class="detail-tag text-muted">{{ acc.last_sync_at ? formatTime(acc.last_sync_at) : '从未同步' }}</span>
        </div>

        <!-- 右侧：操作 -->
        <div class="item-actions">
          <button
            class="btn btn-ghost btn-sm"
            :class="{ 'text-muted': acc.status === 'disabled' }"
            :title="acc.status === 'disabled' ? '启用' : '停用'"
            :disabled="togglingId === acc.id"
            @click="handleToggleStatus(acc)"
          >
            <span v-if="togglingId === acc.id" class="spinner-xs"></span>
            <svg v-else width="14" height="14" viewBox="0 0 14 14" fill="none">
              <!-- 播放(启用) / 暂停(停用) 图标 -->
              <template v-if="acc.status === 'disabled'">
                <path d="M4 3l8 4-8 4V3z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
              </template>
              <template v-else>
                <rect x="3" y="2" width="3" height="10" rx="0.5" stroke="currentColor" stroke-width="1.3"/>
                <rect x="8" y="2" width="3" height="10" rx="0.5" stroke="currentColor" stroke-width="1.3"/>
              </template>
            </svg>
          </button>
          <button class="btn btn-ghost btn-sm" @click="openForm(acc)" title="编辑">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M9.5 1l3.5 3.5-8 8H1.5V9l8-8z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/>
            </svg>
          </button>
          <button class="btn btn-ghost btn-sm" :disabled="syncingIds.has(acc.id)" @click="handleSync(acc.id)" title="立即同步">
            <span v-if="syncingIds.has(acc.id)" class="spinner-xs"></span>
            <svg v-else width="16" height="16" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg"><path d="M882.548896 835.161121c-5.650697 0-10.233062-4.582365-10.233062-10.233062L872.315834 615.20964l-192.749956 0c-5.650697 0-10.233062-4.582365-10.233062-10.233062s4.582365-10.233062 10.233062-10.233062l202.983018 0c5.650697 0 10.233062 4.582365 10.233062 10.233062l0 219.950458C892.781958 830.578756 888.200616 835.161121 882.548896 835.161121z" fill="currentColor" stroke="currentColor" stroke-width="28"/><path d="M344.436168 415.054017 141.450081 415.054017c-5.650697 0-10.233062-4.581342-10.233062-10.233062l0-219.949434c0-5.65172 4.582365-10.233062 10.233062-10.233062 5.65172 0 10.233062 4.581342 10.233062 10.233062l0 209.716372 192.753025 0c5.650697 0 10.233062 4.581342 10.233062 10.233062S350.086865 415.054017 344.436168 415.054017z" fill="currentColor" stroke="currentColor" stroke-width="28"/><path d="M510.329453 894.456598c-91.735307 0-180.443675-32.984229-249.785973-92.877317-68.622914-59.270918-114.077152-140.994198-127.988999-230.118028-0.871857-5.583159 2.949168-10.816347 8.532327-11.689227 5.584182-0.870834 10.81737 2.948145 11.689227 8.532327 27.650757 177.12714 178.023556 305.686121 357.553419 305.686121 160.446225 0 299.604612-103.051027 346.276584-256.429277 1.645476-5.40715 7.361665-8.460696 12.769838-6.810103 5.40715 1.644453 8.456602 7.361665 6.810103 12.768815-11.791557 38.754652-29.644157 75.261101-53.058426 108.503203-23.093974 32.788777-51.056839 61.646012-83.10986 85.769432C673.378969 867.946828 593.954035 894.456598 510.329453 894.456598zM874.22123 443.99721c-4.723581 0-8.969279-3.289929-9.997702-8.095375C828.795645 270.243911 679.961898 150.009526 510.329453 150.009526c-156.105361 0-294.118668 99.457176-343.4277 247.487627-1.785669 5.363148-7.581676 8.263198-12.941753 6.474458-5.362124-1.785669-8.261151-7.580652-6.474458-12.941753 12.517081-37.579897 30.786167-72.894194 54.296627-104.963587 23.229051-31.68463 51.072189-59.525722 82.756819-82.750679 65.840544-48.26219 143.918807-73.771167 225.791489-73.771167 44.900629 0 88.872097 7.713682 130.692574 22.926152 40.410362 14.699794 77.849042 36.023448 111.276363 63.378469 67.061348 54.880935 113.917516 131.511219 131.939985 215.774345 1.180895 5.526877-2.340301 10.964726-7.867178 12.146645C875.649765 443.922509 874.929358 443.99721 874.22123 443.99721z" fill="currentColor" stroke="currentColor" stroke-width="60"/></svg>
          </button>
          <button class="btn btn-ghost btn-sm danger-hover" @click="handleDelete(acc)" title="删除">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M2 4h10m-8.67 0V2.67A1.33 1.33 0 0 1 4.67 1.34h4.66a1.33 1.33 0 0 1 1.34 1.33V4m2 0v7.33A1.33 1.33 0 0 1 11.34 13H2.67a1.33 1.33 0 0 1-1.34-1.33V4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <EmptyState
      v-if="!loading && accounts.length === 0"
      icon="inbox"
      title="还没有邮箱账号"
      description="添加你的第一个 IMAP 邮箱，开始代收邮件"
    />

    <!-- 新增向导弹窗 -->
    <AddAccountWizard
      v-if="showForm && !editingAccount"
      @close="closeForm"
      @saved="onSaved"
    />

    <!-- 编辑弹窗（复用原有表单） -->
    <AccountForm
      v-if="showForm && editingAccount"
      :account="editingAccount"
      @close="closeForm"
      @saved="onSaved"
    />
  </div>
</template>

<script setup>
defineOptions({ name: 'AccountManage' })
import { ref, computed, reactive, onMounted, onActivated } from 'vue'
import { useAccountStore } from '@/stores/accountStore'
import { triggerSync } from '@/api/account'
import { useSSE } from '@/composables/useSSE'
import { useToast } from '@/composables/useToast'
import EmptyState from '../components/EmptyState.vue'
import AccountForm from '../components/AccountForm.vue'
import AddAccountWizard from '../components/AddAccountWizard.vue'
import { getProviderByAccount } from '@/data/providers.js'

const toast = useToast()

const accountStore = useAccountStore()

const accounts = computed(() => accountStore.accounts)
const loading = computed(() => accountStore.loading)

// 正在同步中的账号集合（由 SSE 的 account.sync_* 事件驱动，实时反馈进度）
const syncingIds = reactive(new Set())

// 监听账号同步进度（替代手动同步后无反馈的问题）
useSSE({
  onAccountSyncStarted(data) {
    if (data.account_id) syncingIds.add(data.account_id)
  },
  onAccountSyncDone(data) {
    if (data.account_id) syncingIds.delete(data.account_id)
    accountStore.fetchAccounts()
    toast.success(data.mail_count > 0 ? `同步完成，新增 ${data.mail_count} 封邮件` : '同步完成')
  },
  onAccountSyncError(data) {
    if (data.account_id) syncingIds.delete(data.account_id)
    accountStore.fetchAccounts()
    toast.error('同步失败: ' + (data.error || '未知错误'))
  },
  // 账号生命周期：多标签页/多端实时同步账号列表
  onAccountCreated() { accountStore.fetchAccounts() },
  onAccountUpdated() { accountStore.fetchAccounts() },
  onAccountDeleted() { accountStore.fetchAccounts() },
  onAccountStatusChanged() { accountStore.fetchAccounts() },
  // 账号连接健康状态变化（Worker 推送，实时提示连接异常）
  onAccountHealth(data) {
    accountStore.fetchAccounts()
    if (data.status === 'error') {
      toast.error(`账号连接异常 (${data.account_email || ''}): ${data.error || '未知错误'}`)
    }
  },
})

const showForm = ref(false)
const editingAccount = ref(null)
const togglingId = ref(null)

// --- 数据获取 ---
onMounted(async () => {
  await accountStore.fetchAccounts()
})

onActivated(() => {
  // 从缓存恢复时刷新账号列表
  accountStore.fetchAccounts()
})

// --- 操作 ---
function openForm(account) {
  editingAccount.value = account || null
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingAccount.value = null
}

function onSaved() {
  closeForm()
  accountStore.fetchAccounts() // 刷新列表
}

async function handleSync(id) {
  try {
    await triggerSync(id)
    // 触发后由 SSE 的 account.sync_started/done/error 实时反馈进度
  } catch (e) {
    toast.error('触发失败: ' + e.message)
  }
}

async function handleToggleStatus(acc) {
  const newStatus = acc.status === 'disabled' ? 'active' : 'disabled'
  const action = newStatus === 'active' ? '启用' : '停用'
  if (!await toast.confirm(`确定要${action}邮箱 "${acc.email}" 吗？`)) return

  togglingId.value = acc.id
  try {
    await accountStore.toggleStatus(acc.id, newStatus)
    toast.success(`${action}成功`)
  } catch (e) {
    toast.error(`${action}失败: ${e.message}`)
  } finally {
    togglingId.value = null
  }
}

async function handleDelete(account) {
  if (!await toast.confirm(`确定要删除邮箱 "${account.email}" 吗？\n该操作将同时删除其下所有邮件数据！`)) return
  
  try {
    await accountStore.removeAccount(account.id)
    await accountStore.fetchAccounts()
  } catch (e) {
    toast.error('删除失败: ' + e.message)
  }
}

// --- 工具 ---
// 根据账号信息匹配服务商，返回其 SVG 图标字符串（无匹配则返回空，回退到默认信封图标）
function getProviderIcon(acc) {
  const p = getProviderByAccount(acc)
  return p && p.svgIcon ? p.svgIcon : ''
}

function formatTime(dateStr) {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-lg);
}
.page-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
}

/* ---- 列表模式（Grid 对齐）---- */
.account-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.account-item {
  display: grid;
  grid-template-columns: 240px 1fr auto;
  align-items: center;
  gap: var(--space-lg);
  padding: var(--space-md) var(--space-lg);
  background: var(--bg-primary);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
}
.account-item:hover {
  border-color: var(--primary-200);
  box-shadow: var(--shadow-md);
}
.account-item.is-disabled {
  opacity: 0.55;
}
.account-item.is-disabled .item-icon {
  background: var(--bg-tertiary);
  color: var(--text-tertiary);
}
/* 禁用态下品牌图标仍保持透明背景（整体 opacity 已处理变淡效果） */
.account-item.is-disabled .item-icon.has-brand {
  background: transparent;
  color: inherit;
}

/* 左侧：图标 + 名称 */
.item-left {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  min-width: 0;
}

.item-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: var(--radius-md);
  background: var(--mail-unread-bg);
  color: var(--primary-500);
  flex-shrink: 0;
}
/* 服务商品牌图标：移除背景色与主题色，保留 SVG 自身品牌色 */
.item-icon.has-brand {
  background: transparent;
  color: inherit;
}
.item-icon .icon-svg {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}
.item-icon .icon-svg :deep(svg) {
  width: 28px;
  height: 28px;
}

.item-info {
  min-width: 0;
}
.item-name {
  font-size: var(--font-size-base);
  display: block;
  color: var(--text-primary);
}
.item-email {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 160px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.status-dot.active {
  background: var(--success);
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.35);
}
.status-dot.error {
  background: var(--error);
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.35);
}
.status-dot.disabled {
  background: var(--text-tertiary);
}

/* 中间：详情标签 */
.item-details {
  display: flex;
  align-items: center;
  gap: var(--space-lg);
  min-width: 0;
}

.detail-tag {
  font-size: var(--font-size-xs);
  color: var(--text-secondary);
  white-space: nowrap;
  padding: 2px 0;
}
.detail-tag.text-primary { color: var(--primary-500); }
.detail-tag.text-muted { color: var(--text-tertiary); }

/* 右侧：操作按钮 */
.item-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 2px;
}
.danger-hover:hover { color: var(--error); }
.text-muted { color: var(--text-tertiary); }

/* ---- 空状态 / 加载 ---- */
.loading-state {
  display: flex;
  justify-content: center;
  padding: 60px 20px;
}

@media (max-width: 900px) {
  .account-item {
    grid-template-columns: 1fr auto;
    padding: var(--space-md);
  }
  .item-left {
    grid-column: 1 / -1;
    margin-bottom: var(--space-xs);
  }
  .item-email { max-width: unset; }
  .item-details {
    grid-column: 1 / -1;
    gap: var(--space-md);
    margin-top: var(--space-xs);
    padding-top: var(--space-sm);
    border-top: 1px solid var(--border-light);
    justify-content: space-between;
  }

  .page-header {
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-sm);
  }
}
</style>
