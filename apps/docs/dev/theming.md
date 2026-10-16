# 主题定制

Magicmail 使用 CSS 变量驱动的主题系统，可以轻松自定义外观。

## CSS 变量定义

主题变量位于 `apps/web/src/styles/themes.css`：

### 浅色主题

```css
:root {
  /* 主色调 */
  --color-primary: #646cff;
  --color-primary-hover: #535bf2;

  /* 背景色 */
  --color-bg: #ffffff;
  --color-bg-secondary: #f5f5f5;
  --color-bg-sidebar: rgba(255, 255, 255, 0.72);

  /* 文字色 */
  --color-text: #1a1a2e;
  --color-text-secondary: #666666;

  /* 边框 */
  --color-border: #e0e0e0;

  /* 其他 */
  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.08);
  --radius: 12px;
}
```

### 深色主题

```css
[data-theme="dark"] {
  --color-primary: #7c8aff;
  --color-primary-hover: #646cff;

  --color-bg: #0f0f1a;
  --color-bg-secondary: #1a1a2e;
  --color-bg-sidebar: rgba(15, 15, 26, 0.82);

  --color-text: #e0e0e0;
  --color-text-secondary: #999999;

  --color-border: #2a2a3e;
  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.3);
}
```

## 修改主色调

只需更改 `--color-primary` 和 `--color-primary-hover` 的值即可全局生效。

常用配色方案：

| 风格 | Primary Color | Hover Color |
|------|--------------|-------------|
| 默认紫 | `#646cff` | `#535bf2` |
| 海洋蓝 | `#0077b6` | #005f94` |
| 翡翠绿 | `#2d9d78` | `#237a5c` |
| 玫瑰红 | `#e84a5f` | `#c93a4d` |
| 琥珀橙 | `#f59e0b` | `#d97706` |

## 毛玻璃效果

侧边栏使用 Glassmorphism 效果：

```css
.glass-panel {
  background: var(--color-bg-sidebar);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.12);
}
```

如不需要毛玻璃效果，可将 `backdrop-filter` 移除，改用实心背景色。

## 暗色模式检测

系统自动检测操作系统偏好：

```js
// apps/web/src/composables/useTheme.js
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
```

用户也可以手动覆盖，设置会持久化到 localStorage。
