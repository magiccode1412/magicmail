# 添加 IMAP 功能

本节介绍如何扩展 Magicmail 的 IMAP 能力，例如添加新的邮件操作或同步策略。

## IMAP 模块架构

```
apps/server/imap/
├── client.go      # 底层 IMAP 连接管理
├── fetcher.go     # 邮件拉取与 MIME 解析
└── worker.go      # 后台调度（IDLE + 轮询）
```

## 扩展步骤

### 第一步：底层操作 (`client.go`)

添加新的 IMAP 命令或操作方法：

```go
// 示例：添加邮件夹（文件夹）列表功能
func (c *IMAPClient) ListFolders() ([]string, error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.client == nil {
        return nil, fmt.Errorf("not connected")
    }

    var folders []string
    // 使用 go-imap/v2 的 LIST 命令
    cmd := c.client.List("", "*")
    for cmd.Next() {
        mailbox := cmd.Mailbox()
        folders = append(folders, mailbox.ParsedName().Raw())
    }

    return folders, cmd.Close()
}
```

### 第二步：解析逻辑 (`fetcher.go`)

如果涉及新数据的提取和解析：

```go
// 示例：提取邮件 Flags（标记）
func extractFlags(entity *mime.Entity) []string {
    var flags []string
    // 从 IMAP FETCH 响应中提取 flags
    // ...
    return flags
}
```

### 第三步：调度策略 (`worker.go`)

如果需要后台定时任务或事件驱动：

```go
// 示例：定期清理过期邮件
func (w *Worker) startCleanupTicker() {
    ticker := time.NewTicker(24 * time.Hour)
    go func() {
        for range ticker.C {
            w.cleanupExpiredMails()
        }
    }()
}
```

### 第四步：Handler / Service / Route

按照标准三层架构向上暴露 API：

1. **Service** (`services/`)：调用 IMAP 模块的方法
2. **Handler** (`handlers/`)：接收 HTTP 请求，调用 Service
3. **Route** (`routes/`)：注册路由映射

## 注意事项

- **并发安全**：IMAP 连接不是线程安全的，务必加锁操作
- **连接复用**：使用连接池避免频繁建连/断开
- **错误处理**：IMAP 连接可能随时断开，做好重连机制
- **资源释放**：defer 确保连接正确关闭
- **兼容性**：不同 IMAP 服务器的行为可能有差异，充分测试
