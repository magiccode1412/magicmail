# 前端开发指南

## 环境准备

```bash
cd apps/web
pnpm install
pnpm dev
```

访问 http://localhost:5173，API 请求会自动代理到后端 8080 端口。

## 目录职责说明

```
apps/web/src/
├── main.js             # 应用入口：创建 Vue 实例、挂载插件
├── App.vue             # 根组件：布局框架（侧边栏 + 内容区）
├── router/
│   └── index.js        # 路由配置 + 导航守卫
├── api/
│   ├── index.js        # Axios 实例 + 拦截器（Token 注入）
│   ├── auth.js         # 认证相关 API
│   ├── account.js      # 邮箱 API
│   ├── mail.js         # 邮件 API
│   └── webhook.js      # Webhook API
├── stores/
│   ├── auth.js         # 登录状态 / 用户信息
│   ├── mail.js         # 邮件列表 / 当前邮件
│   ├── account.js      # 邮箱账号列表
│   └── app.js          # 主题 / 侧边栏状态
├── views/
│   ├── LoginView.vue   # 登录页
│   ├── MailListView.vue    # 邮件列表
│   ├── MailDetailView.vue  # 邮件详情
│   ├── SettingsView.vue    # 设置中心
│   └── ...
├── components/
│   ├── AppSidebar.vue      # 侧边导航栏
│   ├── MailItem.vue         # 邮件列表项
│   ├── SearchBar.vue        # 搜索框
│   └── ThemeToggle.vue      # 主题切换按钮
├── composables/
│   ├── useAuth.js           # 认证相关组合函数
│   └── useTheme.js          # 主题相关组合函数
└── styles/
    ├── main.css             # 全局样式重置
    ├── themes.css           # CSS 变量（浅色/深色主题）
    └── components.css       # 组件通用样式
```

## 添加新页面

以「标签管理」页面为例：

### 1. 创建视图组件

```vue
<!-- apps/web/src/views/TagManageView.vue -->
<script setup>
import { ref, onMounted } from 'vue'
import { getTags, createTag } from '@/api/tag'

const tags = ref([])
const newTagName = ref('')

onMounted(async () => {
  const res = await getTags()
  tags.value = res.data.items
})

async function handleCreate() {
  await createTag({ name: newTagName.value })
  newTagName.value = ''
  // 刷新列表...
}
</script>

<template>
  <div class="tag-manage">
    <h2>标签管理</h2>
    <!-- ... -->
  </div>
</template>
```

### 2. 注册路由

```js
// apps/web/src/router/index.js
{
  path: '/settings/tags',
  name: 'TagManage',
  component: () => import('@/views/TagManageView.vue'),
  meta: { requiresAuth: true }
}
```

### 3. 添加导航入口

在 `AppSidebar.vue` 的设置区域添加链接。

### 4. 如需全局状态，创建 Store

```js
// apps/web/src/stores/tag.js
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useTagStore = defineStore('tag', () => {
  const tags = ref([])
  // ...
})
```

## API 封装规范

```js
// apps/web/src/api/tag.js
import request from './index'

export function getTags(params) {
  return request.get('/tags', { params })
}

export function createTag(data) {
  return request.post('/tags', data)
}

export function updateTag(id, data) {
  return request.put(`/tags/${id}`, data)
}

export function deleteTag(id) {
  return request.delete(`/tags/${id}`)
}
```

## 样式规范

- 使用 [BEM 命名](https://getbem.com/)：`.block__element--modifier`
- 颜色值使用 CSS 变量（定义在 `themes.css`），禁止硬编码
- 响应式优先：移动端适配是必须的
- 动画使用 CSS transition/animation，不引入额外动画库
