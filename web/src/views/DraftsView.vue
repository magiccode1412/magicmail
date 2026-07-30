<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="drafts-view">
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

    <!-- 草稿列表区域 -->
    <div class="draft-list-container">
      <!-- 加载状态 -->
      <div v-if="loading && drafts.length === 0" class="loading-state">
        <div class="spinner-lg"></div>
        <p>正在加载...</p>
      </div>

      <!-- 空状态 -->
      <EmptyState
        v-else-if="!loading && drafts.length === 0"
        icon="draft"
        :title="emptyTitle"
        :description="emptyDescription"
      />

      <!-- 草稿列表 -->
      <TransitionGroup v-else name="draft-list" tag="div" class="draft-list">
        <div
          v-for="draft in drafts"
          :key="draft.id"
          class="draft-item card"
          :class="{ 'draft-selected': selectedIds.includes(draft.id) }"
          @click="handleDraftClick(draft)"
        >
          <!-- 选择框 -->
          <label v-if="selectable" class="draft-checkbox" @click.stop>
            <input
              type="checkbox"
              :checked="selectedIds.includes(draft.id)"
              @change="toggleSelect(draft.id)"
            />
          </label>
          <div class="draft-item-header">
            <span class="draft-subject">{{ draft.subject || '(无主题)' }}</span>
            <span class="draft-time">{{ formatTime(draft.updated_at) }}</span>
          </div>
          <div class="draft-item-body">
            <span class="draft-to">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" class="draft-icon">
                <path d="M2 4L7 7L12 4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
                <rect x="2" y="3.5" width="10" height="7" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
              </svg>
              {{ formatRecipients(draft.to) }}
            </span>
            <span v-if="draft.preview" class="draft-preview">{{ draft.preview }}</span>
          </div>
          <div class="draft-item-actions">
            <span v-if="draft.account_name" class="draft-account">{{ draft.account_name }}</span>
            <button
              class="draft-delete-btn"
              title="删除草稿"
              @click.stop="handleDelete(draft.id)"
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                <path d="M2 4h10M5 4V3a1 1 0 011-1h2a1 1 0 011 1v1M6 7v3M8 7v3M3 4l.75 6.5A1.5 1.5 0 005.22 11h3.56A1.5 1.5 0 0010.25 9.5L11 4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </button>
          </div>
        </div>
      </TransitionGroup>

      <!-- 加载更多指示器 -->
      <div v-if="loading && drafts.length > 0" class="loading-more">
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
defineOptions({ name: 'Drafts' })
import { ref, computed, watch, onMounted, onActivated, onUnmounted, onDeactivated } from 'vue'
import { useRouter } from 'vue-router'
import { getDrafts, deleteDraft as apiDeleteDraft, batchDeleteDrafts as apiBatchDelete } from '@/api/draft'
import { useAppStore } from '@/stores/appStore'
import { useSSE } from '@/composables/useSSE'
import { useToast } from '@/composables/useToast'
import Pagination from '../components/Pagination.vue'
import EmptyState from '../components/EmptyState.vue'

const toast = useToast()
const router = useRouter()
const appStore = useAppStore()

// --- 筛选状态 ---
const activeFilter = ref('all')
const sortOrder = ref('desc')
const showSortDropdown = ref(false)

// --- 批量选择状态 ---
const selectable = ref(false)
const selectedIds = ref([])
const isAllSelected = computed(() => drafts.value.length > 0 && selectedIds.value.length === drafts.value.length)

const filterTabs = computed(() => {
  return [
    { key: 'all', label: '全部' },
  ]
})

const sortOptions = [
  { value: 'desc', label: '最近修改' },
  { value: 'asc', label: '最早修改' },
]
const sortLabel = computed(() => {
  const opt = sortOptions.find(o => o.value === sortOrder.value)
  return opt ? opt.label : '排序'
})

// 列表数据
const loading = ref(false)
const drafts = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

// 搜索关键词（来自顶部全局搜索框）
const searchKeyword = ref(appStore.searchKeyword)
watch(() => appStore.searchKeyword, (kw) => {
  searchKeyword.value = kw
  fetchDrafts(1)
})

// 空状态文案
const emptyTitle = computed(() => {
  if (searchKeyword.value) return `未找到 "${searchKeyword.value}" 相关草稿`
  return '暂无草稿'
})
const emptyDescription = computed(() => '保存的草稿将显示在这里')

// --- 操作 ---
function handleFilter(filterKey) {
  activeFilter.value = filterKey
  fetchDrafts(1)
}

function handleSortSelect(value) {
  sortOrder.value = value
  showSortDropdown.value = false
  fetchDrafts(1)
}

function toggleSortDropdown() {
  showSortDropdown.value = !showSortDropdown.value
}

function changePage(page) {
  fetchDrafts(page)
}

function editDraft(id) {
  if (selectable.value) return
  router.push({ path: '/compose', query: { draftId: id } })
}

function handleDraftClick(draft) {
  if (selectable.value) return
  editDraft(draft.id)
}

async function handleDelete(id) {
  try {
    await apiDeleteDraft(id)
    drafts.value = drafts.value.filter(d => d.id !== id)
    total.value--
    toast.success('草稿已删除')
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
    selectedIds.value = drafts.value.map(d => d.id)
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
  if (!confirm(`确定要删除选中的 ${count} 个草稿吗？此操作不可撤销。`)) return
  try {
    await apiBatchDelete(selectedIds.value)
    drafts.value = drafts.value.filter(d => !selectedIds.value.includes(d.id))
    total.value -= count
    toast.success(`已删除 ${count} 个草稿`)
    clearSelection()
  } catch (e) {
    toast.error('批量删除失败: ' + e.message)
  }
}

// 处理来自其它标签页/客户端的草稿删除事件（SSE draft.deleted）
// 从本地草稿列表即时移除指定项，无需整列表刷新，实现跨标签页实时一致。
// 幂等：若本地已不存在这些 ID（如发起删除的标签页已自行移除），则直接返回，避免重复扣减计数。
function handleDraftDeleted(ids) {
  if (!ids || !ids.length) return
  const idSet = new Set(ids.map(String))
  const before = drafts.value.length
  drafts.value = drafts.value.filter(d => !idSet.has(String(d.id)))
  const removed = before - drafts.value.length
  if (removed === 0) return // 本地已移除（如发起删除的标签页），幂等退出
  total.value -= removed
  // 同步清理多选集合，避免指向已不存在的草稿
  if (selectedIds.value.length > 0) {
    selectedIds.value = selectedIds.value.filter(id => !idSet.has(String(id)))
  }
}

function formatRecipients(toStr) {
  if (!toStr) return '-'
  try {
    const arr = JSON.parse(toStr)
    return arr.join(', ')
  } catch {
    return toStr
  }
}

function formatTime(timeStr) {
  const d = new Date(timeStr)
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  if (isToday) {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  const yesterday = new Date(now)
  yesterday.setDate(yesterday.getDate() - 1)
  if (d.toDateString() === yesterday.toDateString()) {
    return '昨天'
  }
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

// --- 数据加载 ---
async function fetchDrafts(page = 1) {
  loading.value = true

  try {
    const params = {
      page,
      page_size: pageSize.value,
      sort_order: sortOrder.value,
    }

    if (searchKeyword.value) {
      params.keyword = searchKeyword.value
    }

    const res = await getDrafts(params)
    drafts.value = res.data || []
    total.value = res.total || 0
    currentPage.value = res.page || page
  } catch (e) {
    console.error('[DraftsView] 获取草稿列表失败:', e.message)
  } finally {
    loading.value = false
  }
}

// --- SSE 实时推送（跨标签页一致）---
// ⭐ 必须在 setup 同步阶段注册（与 MailListView / SentView 一致），用于跨标签页实时更新。
// 仅消费 draft.deleted：其它标签页/客户端删除草稿时，本视图从本地列表即时移除，无需整列表刷新。
const { connectionMode: sseMode } = useSSE({
  onDraftDeleted: (data) => {
    handleDraftDeleted(data?.ids || [])
  },
})

watch(sseMode, (mode) => {
  if (mode) appStore.setConnectionMode(mode)
}, { immediate: true })

// --- 生命周期 ---
onMounted(async () => {
  searchKeyword.value = appStore.searchKeyword
  await fetchDrafts(1)
  document.addEventListener('click', handleDocumentClick)
})

function handleDocumentClick(e) {
  if (!e.target.closest('.sort-filter')) showSortDropdown.value = false
}

onActivated(() => {
  searchKeyword.value = appStore.searchKeyword
  fetchDrafts(currentPage.value)
})

onDeactivated(() => {
  // pause
})

onUnmounted(() => {
  document.removeEventListener('click', handleDocumentClick)
})
</script>

<style scoped>
.drafts-view {
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

/* ---- 排序下拉框 ---- */
.sort-filter {
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
}
.filter-btn:hover {
  border-color: var(--primary-200);
  color: var(--text-primary);
}
.btn-arrow {
  flex-shrink: 0;
  transition: transform 0.2s ease;
}
.btn-arrow.open { transform: rotate(180deg); }

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

/* ---- 草稿列表容器 ---- */
.draft-list-container {
  flex: 1;
  min-height: 200px;
}

.draft-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs, 4px);
}

/* ---- 草稿卡片项 ---- */
.draft-item {
  padding: 14px 16px;
  cursor: pointer;
  transition: all var(--transition-fast);
  border: 1px solid transparent;
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.draft-item:hover {
  border-color: var(--primary-200);
  box-shadow: 0 2px 8px rgba(79, 110, 247, 0.08);
}
.draft-item.draft-selected {
  background: var(--mail-selected-bg);
  border-color: var(--primary-300);
}

/* ---- 草稿选择框 ---- */
.draft-checkbox {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 2px;
  cursor: pointer;
}
.draft-checkbox input[type="checkbox"] {
  appearance: none;
  -webkit-appearance: none;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color);
  border-radius: 4px;
  cursor: pointer;
  transition: all var(--transition-fast);
}
.draft-checkbox input[type="checkbox"]:checked {
  background: var(--primary-500);
  border-color: var(--primary-500);
}
.draft-checkbox input[type="checkbox"]:checked::after {
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
.draft-checkbox input[type="checkbox"] { position: relative; }

.draft-item-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.draft-subject {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.draft-time {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  flex-shrink: 0;
}

.draft-item-body {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.draft-to {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 300px;
}

.draft-icon {
  flex-shrink: 0;
  opacity: 0.55;
}

.draft-preview {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.draft-item-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.draft-account {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  background: var(--bg-secondary);
  padding: 2px 8px;
  border-radius: var(--radius-full);
}

.draft-delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text-tertiary);
  cursor: pointer;
  transition: all var(--transition-fast);
  opacity: 0;
}
.draft-item:hover .draft-delete-btn { opacity: 1; }
.draft-delete-btn:hover {
  background: rgba(239, 68, 68, 0.1);
  color: var(--error);
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
.draft-list-enter-active { transition: all 0.25s ease-out; }
.draft-list-leave-active { transition: all 0.15s ease-in; }
.draft-list-enter-from {
  opacity: 0;
  transform: translateX(-16px);
}
.draft-list-leave-to {
  opacity: 0;
  transform: translateX(16px);
}
</style>
