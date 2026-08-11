xu'yao<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="about-view">
    <h2 class="page-title">关于</h2>

    <div class="about-sections">
      <!-- 应用信息 -->
      <section class="settings-section card">
        <h3 class="section-title">应用信息</h3>
        <p class="section-desc">查看版本与运行状态</p>

        <div class="about-info">
          <div class="info-item">
            <span>应用名称</span>
            <span><strong>Magicmail 魔法邮箱</strong></span>
          </div>
          <div class="info-item">
            <span>当前版本</span>
            <span>v{{ localVersion }}</span>
          </div>
          <div v-if="remoteVersion" class="info-item">
            <span>最新版本</span>
            <span :class="{ 'text-success': versionHasUpdate, 'text-tertiary': !versionHasUpdate }">
              {{ remoteVersion }}
              <span v-if="versionHasUpdate" class="badge badge-success" style="margin-left: 6px;">有新版本</span>
              <span v-else-if="!checkingUpdate && remoteVersion === `v${localVersion}`" class="badge badge-default" style="margin-left: 6px;">已是最新</span>
            </span>
          </div>
          <div class="info-item">
            <span>API 地址</span>
            <span>{{ apiBase }}</span>
          </div>
        </div>

        <div class="setting-actions">
          <button
            class="btn btn-secondary btn-sm"
            :disabled="checkingUpdate"
            @click="doCheckUpdate(true)"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" style="animation: checkingUpdate ? spin 0.8s linear infinite : none;">
              <path d="M7 2v5l3 3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.2"/>
            </svg>
            {{ checkingUpdate ? '检查中...' : '检查更新' }}
          </button>
          <a
            href="https://magicmail1412.netlify.app/"
            target="_blank"
            rel="noopener"
            class="btn btn-secondary btn-sm"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M7 1.5L12 4.5V9.5L7 12.5L2 9.5V4.5L7 1.5Z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
              <path d="M7 1.5V7M7 7L12 4.5M7 7L2 4.5" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
            </svg>
            更新日志
          </a>
          <a
            v-if="versionHasUpdate && versionDownloadUrl"
            :href="versionDownloadUrl"
            target="_blank"
            rel="noopener"
            class="btn btn-primary btn-sm"
          >
            前往下载
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style="display: inline-block; vertical-align: middle;">
              <path d="M9 3L4.5 7.5M4.5 7.5L9 12M4.5 7.5H1.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            </svg>
          </a>
          <button class="btn btn-secondary btn-sm" @click="clearCache">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M11.5 4l-5-2-4 8 5 2 4-8z" stroke="currentColor" stroke-width="1.2"/>
              <circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="1.2"/>
            </svg>
            清除缓存
          </button>
        </div>
      </section>

      <!-- 作者与版权 -->
      <section class="settings-section card">
        <h3 class="section-title">作者与版权</h3>
        <p class="section-desc">本项目由魔法代码开发并开源</p>

        <div class="about-info">
          <div class="info-item">
            <span>作者</span>
            <span><strong>魔法代码 (magiccode)</strong></span>
          </div>
          <div class="info-item">
            <span>GitHub 主页</span>
            <a :href="githubProfile" target="_blank" rel="noopener" class="link">{{ githubProfile }}</a>
          </div>
          <div class="info-item">
            <span>项目仓库</span>
            <a :href="githubRepo" target="_blank" rel="noopener" class="link">{{ githubRepo }}</a>
          </div>
          <div class="info-item">
            <span>开源协议</span>
            <span>AGPL-3.0-or-later</span>
          </div>
        </div>

        <p class="copyright">
          Copyright © 2026 magiccode (魔法代码). All rights reserved.<br />
          基于 AGPL-3.0 协议开源，使用时请保留版权与许可声明。
        </p>
      </section>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'About' })
import { useToast } from '@/composables/useToast'
import { useUpdateCheck } from '@/composables/useUpdateCheck'

const toast = useToast()

// --- 版本更新检测 ---
const {
  latestVersion: remoteVersion,
  currentVersion: localVersion,
  hasUpdate: versionHasUpdate,
  downloadUrl: versionDownloadUrl,
  loading: checkingUpdate,
  checkUpdate: doCheckUpdate,
} = useUpdateCheck()

const apiBase = window.location.origin + import.meta.env.BASE_URL + 'api/v1'

const githubProfile = 'https://github.com/magiccode1412'
const githubRepo = 'https://github.com/magiccode1412/magicmail'

// --- 缓存操作 ---
async function clearCache() {
  if (!await toast.confirm('确定要清除所有本地缓存数据吗？')) return

  try {
    // 清除 localStorage 偏好设置以外的缓存
    const keysToRemove = []
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key !== 'theme-mode' && key?.startsWith('mail-')) {
        keysToRemove.push(key)
      }
    }
    keysToRemove.forEach(k => localStorage.removeItem(k))

    // 清除 Service Worker 缓存
    if ('caches' in window) {
      const cacheNames = await caches.keys()
      await Promise.all(cacheNames.map(name => caches.delete(name)))
    }

    toast.success('缓存已清除')
    location.reload()
  } catch (e) {
    toast.error('清除失败: ' + e.message)
  }
}
</script>

<style scoped>
.page-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  margin-bottom: var(--space-lg);
}

.about-sections {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
}

.settings-section { padding: var(--space-xl); }

.section-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: var(--space-xs);
}
.section-desc {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  margin-bottom: var(--space-lg);
}

/* ---- 关于信息 ---- */
.about-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-md) 0;
  border-bottom: 1px solid var(--border-light);
  margin-bottom: var(--space-md);
}
.info-item {
  display: flex;
  justify-content: space-between;
  gap: var(--space-md);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}
.info-item .text-success { color: var(--success); }
.info-item .text-tertiary { color: var(--text-tertiary); }

.link {
  color: var(--primary-500);
  text-decoration: none;
  word-break: break-all;
}
.link:hover { color: var(--primary-600); text-decoration: underline; }

.copyright {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  line-height: 1.6;
  margin: 0;
}

.setting-actions {
  display: flex;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: var(--font-weight-medium);
  border-radius: var(--radius-full);
  line-height: 1.5;
  white-space: nowrap;
}
.badge-success { background: var(--success-light); color: var(--success); }
.badge-default { background: var(--bg-hover); color: var(--text-tertiary); }

@media (max-width: 480px) {
  .settings-section { padding: var(--space-lg); }
}
</style>
