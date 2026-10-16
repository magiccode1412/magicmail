<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="pagination">
    <!-- 上一页 -->
    <button
      class="btn btn-ghost btn-sm"
      :disabled="currentPage <= 1"
      @click="$emit('change', currentPage - 1)"
    >
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
        <path d="M8 3L4 7l4 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      上一页
    </button>

    <!-- 页码信息 -->
    <div class="page-info">
      <span class="page-current">{{ currentPage }}</span>
      <span class="page-divider">/</span>
      <span class="page-total">{{ totalPages }}</span>
    </div>

    <!-- 下一页 -->
    <button
      class="btn btn-ghost btn-sm"
      :disabled="currentPage >= totalPages"
      @click="$emit('change', currentPage + 1)"
    >
      下一页
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
        <path d="M6 3l4 4-4 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  currentPage: { type: Number, default: 1 },
  pageSize: { type: Number, default: 20 },
  total: { type: Number, default: 0 }
})

defineEmits(['change'])

const totalPages = computed(() => {
  return Math.ceil(props.total / props.pageSize) || 1
})
</script>

<style scoped>
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-sm);
  padding: var(--space-md) 0;
}

.page-info {
  display: flex;
  align-items: baseline;
  gap: 4px;
  padding: 0 var(--space-sm);
  font-size: var(--font-size-sm);
}
.page-current {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}
.page-divider { color: var(--text-tertiary); }
.page-total { color: var(--text-secondary); }

.btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .pagination .btn span { display: none; } /* 只保留箭头 */
}
</style>
