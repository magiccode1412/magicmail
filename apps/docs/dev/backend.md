# 后端开发指南

## 环境准备

```bash
# 确保 Go 版本 >= 1.21
go version

# 安装依赖
cd apps/server
go mod download

# 运行
go run .
```

## 目录职责说明

```
apps/server/
├── main.go              # 应用入口：加载配置 → 初始化DB → 注册路由 → 启动Fiber
├── config/
│   └── config.go        # 从环境变量读取配置，提供默认值
├── database/
│   └── database.go      # GORM 连接 SQLite，执行 AutoMigrate
├── models/
│   ├── user.go          # User 模型
│   ├── account.go       # Account (IMAP 邮箱) 模型
│   ├── mail.go          # Mail 模型
│   ├── attachment.go    # Attachment 模型
│   └── webhook.go       # Webhook 模型
├── handlers/
│   ├── auth_handler.go  # 注册/登录 handler
│   ├── account_handler.go
│   ├── mail_handler.go
│   ├── attachment_handler.go
│   └── webhook_handler.go
├── services/
│   ├── auth_service.go
│   ├── account_service.go
│   ├── mail_service.go
│   └── ...
├── routes/
│   └── routes.go        # 路由注册入口
├── middleware/
│   ├── cors.go
│   └── auth.go          # JWT 鉴权中间件
├── imap/
│   ├── client.go        # IMAP 连接管理
│   ├── fetcher.go       # 邮件拉取 & MIME 解析
│   └── worker.go        # 后台同步调度
├── notifier/
│   └── notifier.go      # Webhook 异步推送引擎
└── embedfs/
    └── embed.go         # //go:embed 嵌入前端 dist
```

## 添加新的 API 接口

以「邮件标签管理」为例：

### 1. 定义 Model

```go
// apps/server/models/tag.go
type Tag struct {
    ID        uint   `gorm:"primaryKey"`
    Name      string `gorm:"uniqueIndex;not null;size:50"`
    Color     string `gorm:"size:7"` // hex color
    CreatedAt time.Time
}
```

在 `database/database.go` 的 AutoMigrate 中注册：

```go
db.AutoMigrate(&Tag{})
```

### 2. 编写 Service

```go
// apps/server/services/tag_service.go
func CreateTag(db *gorm.DB, name, color string) (*Tag, error) {
    tag := Tag{Name: name, Color: color}
    if err := db.Create(&tag).Error; err != nil {
        return nil, err
    }
    return &tag, nil
}
```

### 3. 编写 Handler

```go
// apps/server/handlers/tag_handler.go
func CreateTag(c *fiber.Ctx) error {
    var req struct {
        Name  string `json:"name"`
        Color string `json:"color"`
    }
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"code": 40001, "message": "参数错误"})
    }

    tag, err := services.CreateTag(database.DB, req.Name, req.Color)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"code": 50001, "message": err.Error()})
    }

    return c.JSON(fiber.Map{"code": 0, "data": tag})
}
```

### 4. 注册路由

```go
// apps/server/routes/routes.go
api.Post("/tags", tag_handler.CreateTag)
```

## 统一响应格式

```go
// 成功
c.JSON(fiber.Map{"code": 0, "data": result})

// 错误
c.Status(statusCode).JSON(fiber.Map{"code": errorCode, "message": "错误描述"})
```

## 常用命令

```bash
# 运行
go run .

# 构建
go build -o ../bin/magicmail .

# 运行测试
go test ./...

# 代码格式化
gofmt -w .

# Lint
golangci-lint run
```
