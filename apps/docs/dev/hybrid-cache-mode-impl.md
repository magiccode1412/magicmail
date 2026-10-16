# 方案B：混合缓存模式 - 实施完成

## 🎯 核心设计
**小文件立即缓存 + 大文件懒加载**，完美平衡性能与磁盘空间。

---

## 📁 修改的文件清单

### 1. `apps/server/config/config.go` — 新增配置项
```go
type IMAPConfig struct {
    MaxAttachmentSize int64 // 单附件大小上限（默认 50 MB）
    MinDiskFreeMB     int64 // 最小剩余磁盘空间（默认 1024 MB = 1GB）
    CacheThresholdMB  int64 // 缓存阈值（默认 2 MB）⭐ 小于此值立即缓存
    CacheExpireDays   int   // 缓存过期天数（默认 30 天）
    AutoCacheEnabled  bool  // 是否启用自动缓存（默认 false）⭐ 新增
}
```

**环境变量支持**（单位统一为 **MB**）：
- `MAGICMAIL_AUTO_CACHE` — 是否启用自动缓存（`true`/`1` 开启，默认关闭）
- `MAGICMAIL_MAX_ATTACHMENT_SIZE` (MB)
- `MAGICMAIL_MIN_DISK_FREE` (MB)
- `MAGICMAIL_CACHE_THRESHOLD` (MB)
- `MAGICMAIL_CACHE_EXPIRE_DAYS` (天)

---

### 2. `apps/server/models/attachment.go` — 新增字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `IMAPUID` | uint32 | IMAP 消息 UID ⭐ |
| `PartID` | string | MIME Part ID（如 "1.2"）⭐ |
| `IsCached` | bool | 是否已缓存到本地 ⭐ |
| `CacheExpire` | *time.Time | 缓存过期时间 ⭐ |

**API 响应新增字段**：
- `is_cached` — 前端可据此显示状态图标

---

### 3. `apps/server/imap/fetcher.go` — 核心逻辑变更

#### 修改前：
```
收到邮件 → 解析 MIME → 下载所有附件 → 存 DB/磁盘
```
#### 修改后：
```
收到邮件 → 解析 MIME → 判断附件大小
├─ < 2MB (CacheThreshold) → 立即下载并缓存 ✓
├─ ≥ 2MB 或未知大小     → 只存元数据 (IMAPUID + PartID) ✗ 不下载！
└─ > 50MB (MaxSize)      → 直接跳过，记录日志
```

**关键函数签名变更**：
```go
// 旧版
func (f *Fetcher) parseEntity(entity, result, mailID, baseDir)

// 新版：增加 imapUID 和 partID 参数
func (f *Fetcher) parseEntity(entity, result, mailID, baseDir, imapUID, partID string)
```

**PartID 生成规则**（IMAP RFC 标准）：
```
multipart/mixed
  ├─ [1] multipart/alternative
  │     ├─ [1.1] text/plain          ← 正文
  │     └──[1.2] text/html           ← 正文
  └─ [2] attachment: report.pdf      ← PartID = "2"
        └─ [3] attachment: photo.jpg  ← PartID = "3"
```

**4层保护机制已内置**：
1. **配置层**: `MaxAttachmentSize` 拒绝超大文件
2. **预检查层**: `CacheThreshold` 决定是否懒加载
3. **运行时层**: `io.LimitReader` 限制读取量（可扩展）
4. **磁盘层**: `CheckDiskSpace()` 通过 `syscall.Statfs` 检查剩余空间

---

### 4. `apps/server/services/attachment_service.go` — 业务逻辑重构

**核心方法**：

| 方法 | 功能 |
|------|------|
| `GetByID()` | 获取附件（自动判断缓存/懒加载状态）|
| `GetByMailID()` | 获取列表（含 `is_cached` 字段）|
| `GetMailAndAccount()` | 获取邮件+账号（用于重建 IMAP 连接）|
| `StreamFromIMAP()` | ⭐ 从 IMAP 流式获取指定 MIME 部分 |
| `CacheAttachment()` | 将内容缓存到本地文件系统 |
| `CleanupExpiredCache()` | 清理过期缓存（建议定时任务调用）|
| `ShouldAutoCache()` | ⭐ 判断是否满足自动缓存条件（新增）|
| `CheckDiskSpace()` | ⭐ 检查磁盘剩余空间是否足够（新增）|

**StreamFromIMAP 核心流程**：
```
构建 BODY[PartID] 请求 → IMAP FETCH → 解析 MIME Entity → 返回 io.ReadCloser
                                              ↓
                                        HTTP Response (零拷贝)
```

---

### 5. `apps/server/handlers/attachment_handler.go` — HTTP 层改造

**Download 方法三模式路由**：

```go
func Download(c *fiber.Ctx) error {
    att := GetByID(id)

    if att.IsCached {
        // 模式1: 已缓存 → 直接返回 DB BLOB / 文件
        return c.Send(att.Content) || c.SendFile(att.FilePath)
    }

    if att.IMAPUID > 0 && att.PartID != "" {
        // 模式2: 懒加载 → 建立临时 IMAP 连接 → 流式传输 [+ 可选自动缓存]
        return streamFromIMAP(c, att)
    }

    // 模式3: 异常状态 → 返回 503 错误
    return c.Status(503).JSON("附件暂不可用")
}
```

**streamFromIMAP 详细步骤**：
1. 查询邮件关联的邮箱账号 (`GetMailAndAccount`)
2. 创建临时 IMAP 连接 (`NewIMAPClient`)
3. 认证并选择对应文件夹 (INBOX/Sent)
4. 调用 `Service.StreamFromIMAP()` 获取 `io.ReadCloser`
5. 设置响应头 (`Content-Type`, `Content-Disposition`, `X-Cache-Status: miss`)
6. **自动缓存判断**：调用 `ShouldAutoCache()` 检查是否需要同时缓存
7. 如需缓存：使用 `io.TeeReader` + `io.Pipe` 边传输边写入本地
8. `c.Write(reader)` 零内存拷贝流式输出

---

## 🔧 配置示例

### 生产环境（推荐）
```bash
# 30GB VPS 的保守配置
export MAGICMAIL_AUTO_CACHE=true           # 开启自动缓存
export MAGICMAIL_MAX_ATTACHMENT_SIZE=50     # 50MB 单附件上限
export MAGICMAIL_MIN_DISK_FREE=5120        # 保留 5GB 空闲
export MAGICMAIL_CACHE_THRESHOLD=2         # 2MB 以上懒加载
export MAGICMAIL_CACHE_EXPIRE_DAYS=30      # 30天后清理缓存
```

### 开发环境（宽松模式）
```bash
export MAGICMAIL_AUTO_CACHE=true           # 开启自动缓存
export MAGICMAIL_MAX_ATTACHMENT_SIZE=0     # 不限制大小
export MAGICMAIL_MIN_DISK_FREE=1024        # 只保留 1GB
export MAGICMAIL_CACHE_THRESHOLD=10        # 10MB 以上懒加载（方便调试）
```

### 小机器模式（纯懒加载，不占磁盘）
```bash
export MAGICMAIL_AUTO_CACHE=false          # 关闭自动缓存！
export MAGICMAIL_CACHE_THRESHOLD=0         # 全部懒加载
export MAGICMAIL_MIN_DISK_FREE=512         # 保留 512MB 即可
```

---

## 📊 数据流对比图

### 旧架构（全量下载）
```
用户发送 50MB 附件 → QQ邮箱服务器
                         ↓
              MagicMail 同步时全部下载到本地磁盘
                         ↓
              用户点击下载 → 从本地磁盘返回

问题: 同步100封大附件邮件 = 瞬间占用 5GB 磁盘
```

### 新架构（混合缓存 + 自动缓存）
```
用户发送 50MB 附件 → QQ邮箱服务器
                         ↓
         MagicMail 同步时只保存元数据:
         { filename: "report.pdf", size: 50MB,
           imap_uid: 123, part_id: "2", is_cached: false }

用户点击下载 → MagicMail 临时连接 QQ邮箱 IMAP
             → 请求 BODY[2] 获取 PDF 内容
             → 流式传输给浏览器（不落盘）
             │
             └─ [如果 AUTO_CACHE=true] 同时异步缓存到本地
                    ↓ 下次同一用户再次下载 → 秒传（命中本地缓存）
```

---

## 🚀 性能优势

| 场景 | 旧方案 | 新方案(混合+自动缓存) |
|------|--------|---------------------|
| 同步100封普通邮件(带2MB附件)| 占用 200MB 磁盘 | 占用 200MB 磁盘（立即缓存）|
| 同步10封大附件邮件(50MB每个)| ❌ 爆磁盘(500MB) | ✅ 仅占几 KB 元数据 |
| 用户首次下载50MB附件 | 秒传（本地有）| 流式传输(~5-10s)，同时后台缓存 |
| 用户再次下载50MB附件 | 秒传 | 秒传（已缓存，或 AUTO_CACHE=true 时秒传）|
| 30天未访问的大附件 | 永久占磁盘 | 自动清理释放空间 |
| 小机器部署 | ❌ 磁盘不够用 | ✅ AUTO_CACHE=false 零磁盘占用 |

---

## ⚠️ 注意事项

### 1. IMAP 连接开销
每次懒加载下载需要建立临时 IMAP 连接（~100ms 开销）。对于频繁访问的场景，可以启用自动缓存。

### 2. 自动缓存机制
- **默认关闭**（`MAGICMAIL_AUTO_CACHE=false`），需手动开启
- 使用 `io.TeeReader` + `io.Pipe` 实现流式传输的同时异步写入缓存
- 不阻塞用户的下载体验（缓存写入在后台 goroutine 完成）

### 3. IMAP 服务器兼容性
部分老旧 IMAP 服务器可能不支持 `BODY[partID]` 精确获取。如遇兼容性问题，可回退为获取完整 `BODY[]` 再解析。

### 4. 缓存清理机制
需要添加定时任务定期调用 `CleanupExpiredCache()`。可在 Worker 启动时启动一个 goroutine：

```go
// 在 worker.go 的 StartWorkers 中添加
go func() {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        cleaned, _ := attachmentService.CleanupExpiredCache()
        log.Printf("🧹 清理了 %d 个过期附件缓存", cleaned)
    }
}()
```

### 5. POP3 协议限制
POP3 不支持部分获取，POP3 账号仍需使用旧的全量下载模式。当前代码已通过 `IMAPUID == 0` 自动降级。

### 6. 环境变量单位
附件相关的环境变量（`MAX_ATTACHMENT_SIZE`、`MIN_DISK_FREE`、`CACHE_THRESHOLD`）单位均为 **MB**，无需手动换算字节。

---

## 🔄 回滚方案

如需回退到旧的全量下载模式：
```bash
# 方式1: 设置极大的缓存阈值（等于全量缓存）
export MAGICMAIL_CACHE_THRESHOLD=999999

# 方式2: 关闭自动缓存 + 设置极大阈值（纯懒加载）
export MAGICMAIL_AUTO_CACHE=false
export MAGICMAIL_CACHE_THRESHOLD=0

# 方式3: 代码层面注释掉 shouldLazyLoad 分支（fetcher.go L389-L401）
```

---

## 📝 后续优化方向（可选）

1. **LRU 缓存策略**：基于访问频率而非仅时间过期
2. **预取/预缓存**：用户查看邮件详情页时预加载前 2 个附件
3. **CDN 分流**：大附件上传到 OSS/S3，本地只存 URL
4. **压缩缓存**：对文本类附件(PDF/TXT)进行 gzip 压缩存储
5. **带宽限速**：`io.LimitReader` 包装，避免单个下载占用全部带宽

---

## ✅ 测试验证清单

- [ ] 同步带 1MB 小附件的邮件 → 应该立即缓存到本地
- [ ] 同步带 10MB 大附件的邮件 → 应该只存元数据，不下载
- [ ] 下载已缓存的小附件 → 直接从本地返回
- [ ] 下载未缓存的大附件 → 从 IMAP 流式传输
- [ ] 再次下载刚才的大附件（AUTO_CACHE=true）→ 应该秒传（已自动缓存）
- [ ] 发送超过 50MB 的附件邮件 → 应被拒绝并记录日志
- [ ] 模拟磁盘空间不足 → 应跳过缓存并记录警告
- [ ] 等待 30 天后 → 过期缓存应被自动清理
- [ ] AUTO_CACHE=false → 下载大附件后不会自动缓存到本地
