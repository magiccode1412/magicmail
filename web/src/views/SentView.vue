<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="sent-view">
    <!-- 顶部功能区（进入多选模式后吸顶固定，不随列表滚动） -->
    <div class="mail-list-header" :class="{ 'is-sticky': selectable }">
    <!-- 批量操作工具栏 -->
    <transition name="fade">
      <div v-if="selectable" class="batch-bar">
        <div class="batch-info">
          <label class="select-all-label" @click.stop>
            <input
              type="checkbox"
              :checked="isAllSelected"
              @change="toggleSelectAll"
            />
            已选 {{ selectedIds.length }} 项
          </label>
        </div>
        <div class="batch-actions">
          <button class="btn-batch-danger" @click="handleBatchDelete">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M2 4h10m-8.67 0V2.67A1.33 1.33 0 0 1 4.67 1.34h4.66a1.33 1.33 0 0 1 1.34 1.33V4m2 0v7.33A1.33 1.33 0 0 1 11.34 13H2.67a1.33 1.33 0 0 1-1.34-1.33V4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            全部删除
          </button>
          <button class="btn-batch-text" @click="clearSelection">取消选择</button>
        </div>
      </div>
    </transition>

    <!-- 筛选标签栏 -->
    <div class="filter-bar">
      <div class="filter-tabs">
        <button
          v-for="tab in filterTabs"
          :key="tab.key"
          class="filter-tab"
          :class="{ active: activeFilter === tab.key }"
          @click="handleFilter(tab.key)"
        >
          {{ tab.label }}
          <span v-if="tab.count !== undefined" class="tab-count">{{ tab.count }}</span>
        </button>
      </div>

      <div class="filter-actions">
        <!-- 选择模式开关 -->
        <button
          class="filter-btn select-mode-btn"
          :class="{ active: selectable }"
          @click="toggleSelectable"
          title="批量选择"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <rect x="2" y="2.5" width="10" height="9" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
            <path d="M4.5 7h5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
          </svg>
          {{ selectable ? '取消' : '选择' }}
        </button>

        <!-- 账号筛选器 -->
        <div class="account-filter">
          <button
            class="filter-btn"
            :class="{ active: selectedAccountId !== '' }"
            @click.stop="toggleAccountDropdown"
          >
            <span class="btn-icon-sm">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                <rect x="1" y="3" width="12" height="9" rx="2" stroke="currentColor" stroke-width="1.2"/>
                <path d="M4.5 7h5M7 4.5v5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              </svg>
            </span>
            {{ selectedAccountLabel }}
            <svg width="10" height="6" viewBox="0 0 10 6" fill="none" class="btn-arrow" :class="{ open: showAccountDropdown }">
              <path d="M1 1L5 5L9 1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>

          <!-- 下拉面板 -->
          <transition name="dropdown">
            <div v-if="showAccountDropdown" class="account-dropdown card">
              <button
                v-for="opt in accountOptions"
                :key="opt.id"
                class="account-option"
                :class="{ active: selectedAccountId === opt.id }"
                @click="handleAccountSelect(opt.id)"
              >
                <span class="option-label">{{ opt.label }}</span>
                <span v-if="opt.email && opt.email !== opt.label" class="option-email">{{ opt.email }}</span>
              </button>
            </div>
          </transition>
        </div>

        <!-- 排序选择器 -->
        <div class="sort-filter">
          <button
            class="filter-btn"
            @click.stop="toggleSortDropdown"
          >
            {{ sortLabel }}
            <svg width="10" height="6" viewBox="0 0 10 6" fill="none" class="btn-arrow" :class="{ open: showSortDropdown }">
              <path d="M1 1L5 5L9 1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>

          <transition name="dropdown">
            <div v-if="showSortDropdown" class="sort-dropdown card">
              <button
                v-for="opt in sortOptions"
                :key="opt.value"
                class="account-option"
                :class="{ active: sortOrder === opt.value }"
                @click="handleSortSelect(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
          </transition>
        </div>
      </div>
    </div>
    </div>

    <!-- 邮件列表区域 -->
    <div class="mail-list-container" ref="listContainer">
      <!-- 加载状态 -->
      <div v-if="loading && mails.length === 0" class="loading-state">
        <div class="spinner-lg"></div>
        <p>正在加载...</p>
      </div>

      <!-- 空状态 -->
      <EmptyState
        v-else-if="!loading && mails.length === 0"
        icon="sent"
        :title="emptyTitle"
        :description="emptyDescription"
      />

      <!-- 邮件列表 -->
      <TransitionGroup v-else name="mail-list" tag="div" class="mail-list">
        <MailItem
          v-for="mail in mails"
          :key="mail.id"
          :mail="mail"
          :selectable="selectable"
          :selected="selectedIds.includes(mail.id)"
          @click="openMail(mail.id)"
          @toggle-select="toggleSelect"
          @delete="handleDelete(mail.id)"
        />
      </TransitionGroup>

      <!-- 加载更多指示器 -->
      <div v-if="loading && mails.length > 0" class="loading-more">
        <span class="spinner"></span>
        加载中...
      </div>
    </div>

    <!-- 分页器 -->
    <Pagination
      v-if="total > 0"
      :current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @change="changePage"
    />
  </div>
</template>

<script setup>
defineOptions({ name: 'Sent' })
import { ref, computed, watch, onMounted, onActivated, onUnmounted, onDeactivated } from 'vue'
import { useRouter } from 'vue-router'
import { useMailStore } from '@/stores/mailStore'
import { useAccountStore } from '@/stores/accountStore'
import { useAppStore } from '@/stores/appStore'
import { useToast } from '@/composables/useToast'
import { useMailStream } from '@/composables/useSSE'
import MailItem from '../components/MailItem.vue'
import Pagination from '../components/Pagination.vue'
import EmptyState from '../components/EmptyState.vue'

const toast = useToast()
const router = useRouter()
const mailStore = useMailStore()
const accountStore = useAccountStore()
const appStore = useAppStore()

// --- 筛选状态 ---
const activeFilter = ref('all')
const sortOrder = ref('desc')
const selectedAccountId = ref('')
const showAccountDropdown = ref(false)
const showSortDropdown = ref(false)

// --- 批量选择状态 ---
const selectable = ref(false)
const selectedIds = ref([])
const isAllSelected = computed(() => mails.value.length > 0 && selectedIds.value.length === mails.value.length)

const filterTabs = computed(() => {
  const stats = mailStore.stats || {}
  return [
    { key: 'all', label: '全部' },
    { key: 'attachment', label: '有附件' },
  ]
})

// 账号筛选选项
const accountOptions = computed(() => {
  const accounts = accountStore.accounts || []
  return [
    { id: '', label: '所有邮箱', email: '' },
    ...accounts.map(a => ({
      id: String(a.id),
      label: a.label || a.email,
      email: a.email,
    })),
  ]
})

const selectedAccountLabel = computed(() => {
  const opt = accountOptions.value.find(o => o.id === selectedAccountId.value)
  return opt ? opt.label : '所有邮箱'
})

const sortOptions = [
  { value: 'desc', label: '最新优先' },
  { value: 'asc', label: '最旧优先' },
]
const sortLabel = computed(() => {
  const opt = sortOptions.find(o => o.value === sortOrder.value)
  return opt ? opt.label : '排序'
})

// 列表数据
const loading = computed(() => mailStore.loading)
const mails = computed(() => mailStore.mails)
const total = computed(() => mailStore.total)
const currentPage = computed(() => mailStore.currentPage)
const pageSize = computed(() => mailStore.pageSize)

// 空状态文案
const emptyTitle = computed(() => {
  if (appStore.searchKeyword) return `未找到 "${appStore.searchKeyword}" 相关邮件`
  return '暂无已发送邮件'
})
const emptyDescription = computed(() => '发送的邮件将显示在这里')

// 顶部全局搜索框：关键词变化时在「已发送」列表内就地筛选
watch(() => appStore.searchKeyword, (kw) => {
  mailStore.setFilter('keyword', kw)
})

// --- 操作 ---
async function handleFilter(filterKey) {
  activeFilter.value = filterKey

  let hasAttachment = null
  if (filterKey === 'attachment') hasAttachment = true

  mailStore.setFilter('has_attachment', hasAttachment)
}

function handleAccountSelect(accountId) {
  selectedAccountId.value = accountId
  showAccountDropdown.value = false
  mailStore.setFilter('account_id', accountId || '')
  mailStore.fetchMails(1)
}

function toggleAccountDropdown() {
  showSortDropdown.value = false
  showAccountDropdown.value = !showAccountDropdown.value
}

function handleSortSelect(value) {
  sortOrder.value = value
  showSortDropdown.value = false
  mailStore.setFilter('sort_order', value)
  mailStore.fetchMails(1)
}

function toggleSortDropdown() {
  showAccountDropdown.value = false
  showSortDropdown.value = !showSortDropdown.value
}

function changePage(page) {
  mailStore.fetchMails(page)
}

function openMail(id) {
  router.push({ name: 'MailReader', params: { id } })
}

async function handleDelete(id) {
  if (!confirm('确定要删除这封邮件吗？')) return
  try {
    const res = await mailStore.deleteMail(id)
    if (res?.deleted_from_server) {
      toast.success('邮件已删除，并已从源服务器同步删除')
    } else if (res?.server_delete_error) {
      toast.warning('邮件已本地删除，但源服务器同步失败: ' + res.server_delete_error)
    } else {
      toast.success('邮件已删除')
    }
  } catch (e) {
    toast.error('删除失败: ' + e.message)
  }
}

// --- 批量选择操作 ---
function toggleSelectable() {
  selectable.value = !selectable.value
  if (!selectable.value) clearSelection()
}

function toggleSelect(id) {
  const idx = selectedIds.value.indexOf(id)
  if (idx === -1) {
    selectedIds.value.push(id)
  } else {
    selectedIds.value.splice(idx, 1)
  }
}

function toggleSelectAll(e) {
  if (e.target.checked) {
    selectedIds.value = mails.value.map(m => m.id)
  } else {
    clearSelection()
  }
}

function clearSelection() {
  selectedIds.value = []
  selectable.value = false
}

async function handleBatchDelete() {
  if (selectedIds.value.length === 0) return
  const count = selectedIds.value.length
  if (!confirm(`确定要删除选中的 ${count} 封邮件吗？此操作不可撤销。`)) return
  try {
    const res = await mailStore.batchDeleteMails(selectedIds.value)

    // 分析同步删除结果
    if (res?.server_sync_result) {
      let serverSuccess = 0
      let serverFailed = 0
      Object.values(res.server_sync_result).forEach(r => {
        if (r?.deleted_from_server) serverSuccess++
        else if (r?.server_delete_error) serverFailed++
      })

      if (serverSuccess > 0 && serverFailed === 0) {
        toast.success(`已删除 ${count} 封邮件，并已从源服务器同步删除`)
      } else if (serverFailed > 0 && serverSuccess === 0) {
        toast.warning(`已删除 ${count} 封邮件（本地），但源服务器同步全部失败`)
      } else if (serverFailed > 0) {
        toast.success(`已删除 ${count} 封邮件，其中 ${serverSuccess} 封已从源服务器同步删除`)
      }
    } else {
      toast.success(`已删除 ${count} 封邮件`)
    }

    clearSelection()
  } catch (e) {
    toast.error('批量删除失败: ' + e.message)
  }
}

// --- 生命周期 ---
let refreshTimer = null

// ⭐ SSE 必须在 setup 同步阶段注册（同 MailListView），用于跨标签页实时更新
const { connectionMode: sseMode } = useMailStream(() => {
  // 收到新邮件/删除等事件时自动刷新当前「已发送」列表与统计
  mailStore.fetchMails(mailStore.currentPage)
  mailStore.fetchStats()
}, {
  onFallback: () => {
    // SSE 失败后重启轮询（备用机制）
    console.log('[Sent] SSE 不可用，使用轮询模式')
    startRefreshTimer()
  }
})

watch(sseMode, (mode) => {
  if (mode) appStore.setConnectionMode(mode)
}, { immediate: true })

onMounted(async () => {
  if (accountStore.accounts.length === 0) {
    await accountStore.fetchAccounts()
  }

  // 固定筛选 folder=sent 并应用搜索关键词
  mailStore.filters.folder = 'sent'
  mailStore.filters.keyword = appStore.searchKeyword
  await mailStore.fetchMails(1)
  mailStore.fetchStats()
  startRefreshTimer()

  document.addEventListener('click', handleDocumentClick)
})

function handleDocumentClick(e) {
  if (!e.target.closest('.account-filter')) showAccountDropdown.value = false
  if (!e.target.closest('.sort-filter')) showSortDropdown.value = false
}

onActivated(() => {
  mailStore.filters.folder = 'sent'
  mailStore.filters.keyword = appStore.searchKeyword
  mailStore.fetchMails(mailStore.currentPage)
  mailStore.fetchStats()
  startRefreshTimer()
})

onDeactivated(() => {
  stopRefreshTimer()
})

onUnmounted(() => {
  stopRefreshTimer()
  document.removeEventListener('click', handleDocumentClick)
})

function startRefreshTimer() {
  stopRefreshTimer()
  refreshTimer = setInterval(() => {
    mailStore.fetchMails(mailStore.currentPage)
    mailStore.fetchStats()
  }, 60000)
}

function stopRefreshTimer() {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}
</script>

<style scoped>
.sent-view {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  min-height: calc(100vh - var(--header-height) - 48px);
}

/* ---- 顶部功能区包裹层 ---- */
.mail-list-header {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

/* 进入多选模式后吸顶固定，不随列表滚动 */
.mail-list-header.is-sticky {
  position: sticky;
  top: 0;
  z-index: 20;
  /* 与内容区卡片背景一致，避免列表内容在吸顶区下方透出 */
  background: var(--bg-primary);
  /* 吸顶时与下方列表留出间距，但不额外撑高整体 */
  padding-bottom: var(--space-md);
  margin-bottom: calc(-1 * var(--space-md));
}

/* ---- 批量操作工具栏 ---- */
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: var(--batch-bar-bg, var(--primary-50, rgba(79, 110, 247, 0.06)));
  border: 1px solid var(--batch-bar-border, var(--primary-200, rgba(79, 110, 247, 0.2)));
  border-radius: var(--radius-md);
}
.batch-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: var(--font-size-sm);
  color: var(--text-primary);
}
.select-all-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-weight: var(--font-weight-medium);
}
.select-all-label input[type="checkbox"] {
  appearance: none;
  -webkit-appearance: none;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color);
  border-radius: 4px;
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
  flex-shrink: 0;
}
.select-all-label input[type="checkbox"]:checked {
  background: var(--primary-500);
  border-color: var(--primary-500);
}
.select-all-label input[type="checkbox"]:checked::after {
  content: '';
  position: absolute;
  left: 5px;
  top: 2px;
  width: 4px;
  height: 8px;
  border: solid #fff;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}
.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.btn-batch-danger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: var(--font-size-sm);
  font-family: inherit;
  color: #fff;
  background: var(--error);
  border: none;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.btn-batch-danger:hover { background: #dc2626; }
.btn-batch-text {
  padding: 6px 14px;
  font-size: var(--font-size-sm);
  font-family: inherit;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.btn-batch-text:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.select-mode-btn.active {
  border-color: var(--primary-300);
  color: var(--primary-500);
  background: var(--mail-unread-bg);
}
.fade-enter-active { transition: opacity 0.2s ease-out; }
.fade-leave-active { transition: opacity 0.15s ease-in; }
.fade-enter-from,
.fade-leave-to { opacity: 0; }

/* ---- 筛选栏 ---- */
.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: var(--space-sm) 0;
  border-bottom: 1px solid var(--border-light);
  flex-wrap: wrap;
}

.filter-tabs {
  display: flex;
  gap: var(--space-xs);
}

.filter-actions {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.filter-tab {
  position: relative;
  padding: 8px 16px;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  background: none;
  border: none;
  border-radius: var(--radius-full);
  cursor: pointer;
  transition: all var(--transition-fast);
  white-space: nowrap;
}
.filter-tab:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.filter-tab.active {
  color: var(--primary-500);
  background: var(--mail-unread-bg);
}
.filter-tab.active::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 20px;
  height: 2.5px;
  background: linear-gradient(90deg, var(--primary-500), var(--primary-400));
  border-radius: 2px;
}

.tab-count {
  margin-left: 4px;
  padding: 1px 7px;
  font-size: 11px;
  font-weight: var(--font-weight-semibold);
  background: var(--error-light);
  color: var(--error);
  border-radius: var(--radius-full);
  line-height: 1.4;
}

/* ---- 账号筛选下拉框 ---- */
.account-filter {
  position: relative;
}

.filter-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: var(--font-size-sm);
  font-family: inherit;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  outline: none;
  transition: all var(--transition-fast);
  white-space: nowrap;
  max-width: 200px;
}
.filter-btn:hover {
  border-color: var(--primary-200);
  color: var(--text-primary);
}
.filter-btn.active {
  border-color: var(--primary-300);
  color: var(--primary-500);
  background: var(--mail-unread-bg);
}

.btn-icon-sm {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  opacity: 0.7;
}
.btn-arrow {
  flex-shrink: 0;
  transition: transform 0.2s ease;
}
.btn-arrow.open { transform: rotate(180deg); }

/* 下拉面板 */
.account-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 220px;
  max-height: 320px;
  overflow-y: auto;
  padding: 4px;
  z-index: 50;
  box-shadow: var(--shadow-lg), 0 8px 24px rgba(0,0,0,0.1);
}

/* 排序下拉框 */
.sort-filter {
  position: relative;
}
.sort-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 110px;
  width: max-content;
  padding: 4px;
  z-index: 50;
  box-shadow: var(--shadow-lg), 0 8px 24px rgba(0,0,0,0.1);
}

/* ---- 移动端适配 ---- */
@media (max-width: 640px) {
  .account-filter,
  .sort-filter {
    max-width: calc(100vw - 24px);
    min-width: 0;
  }
  .filter-btn { max-width: unset; }

  .account-dropdown,
  .sort-dropdown {
    left: 0;
    right: auto;
    width: auto;
    max-width: calc(100vw - 16px);
  }
}

.account-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 9px 12px;
  background: none;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-family: inherit;
  font-size: var(--font-size-sm);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s;
}
.account-option:hover { background: var(--bg-hover); }
.account-option.active {
  background: var(--mail-unread-bg);
  color: var(--primary-500);
}

.option-label {
  font-weight: var(--font-weight-medium);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.option-email {
  font-size: 11px;
  color: var(--text-tertiary);
  flex-shrink: 0;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 下拉动画 */
.dropdown-enter-active { animation: dropdown-in 0.18s ease-out; }
.dropdown-leave-active { animation: dropdown-out 0.12s ease-in; }
@keyframes dropdown-in {
  from { opacity: 0; transform: translateY(-6px) scale(0.97); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}
@keyframes dropdown-out {
  from { opacity: 1; transform: translateY(0) scale(1); }
  to   { opacity: 0; transform: translateY(-4px) scale(0.98); }
}

/* ---- 邮件列表容器 ---- */
.mail-list-container {
  flex: 1;
  min-height: 200px;
}

.mail-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs, 4px);
}

/* ---- 加载状态 ---- */
.loading-state, .loading-more {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-sm);
  padding: 60px 20px;
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
}
.spinner-lg {
  width: 32px;
  height: 32px;
  border-width: 2.5px;
}

.loading-more { padding: 20px; }

/* ---- 列表过渡动画 ---- */
.mail-list-enter-active { transition: all 0.25s ease-out; }
.mail-list-leave-active { transition: all 0.15s ease-in; }
.mail-list-enter-from {
  opacity: 0;
  transform: translateX(-16px);
}
.mail-list-leave-to {
  opacity: 0;
  transform: translateX(16px);
}
</style>
