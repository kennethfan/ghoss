# GHoss - GitHub Object Storage Tool

## 项目概述

使用 GitHub 仓库作为对象存储的工具，通过 GitHub API 直接操作仓库文件来实现文件的上传、下载和管理。

**用途**：个人项目/学习用途  
**语言**：Go

---

## 核心功能

1. **文件上传**：将本地文件推送到 GitHub 仓库
2. **文件下载**：从 GitHub 仓库拉取文件到本地
3. **文件列表**：查看仓库中的文件目录
4. **文件删除**：删除仓库中的文件
5. **获取访问链接**：生成文件的公开访问 URL

---

## 技术方案

### 1. GitHub API 使用

- **上传文件**：`PUT /repos/{owner}/{repo}/contents/{path}`
- **下载文件**：`GET /repos/{owner}/{repo}/contents/{path}`
- **列出目录**：`GET /repos/{owner}/{repo}/contents/{path}`
- **删除文件**：`DELETE /repos/{owner}/{repo}/contents/{path}`
- **文件访问**：通过 `https://raw.githubusercontent.com/{owner}/{repo}/{branch}/{path}` 直接访问

### 2. 文件存储方式

- 文件通过 Base64 编码存储在 GitHub 仓库中
- 使用 Git Blob 保存文件内容
- 支持的最大文件大小：100MB（普通文件）

### 3. 项目结构

```
ghoss/
├── cmd/
│   └── ghoss/
│       └── main.go          # CLI 入口
├── internal/
│   ├── github/
│   │   └── client.go        # GitHub API 客户端
│   ├── storage/
│   │   └── storage.go       # 存储逻辑
│   └── config/
│       └── config.go        # 配置管理
├── pkg/
│   └── models/
│       └── file.go          # 数据模型
├── go.mod
├── go.sum
├── README.md
└── .gitignore
```

### 4. CLI 命令设计

```bash
ghoss init              # 初始化配置（设置 token、owner、repo）
ghoss upload <file>     # 上传文件
ghoss download <path>   # 下载文件
ghoss ls [path]         # 列出文件
ghoss rm <path>         # 删除文件
ghoss url <path>        # 获取访问链接
```

---

## 技术限制

### GitHub 作为对象存储的限制

1. **单个文件大小限制**：
   - 普通文件：最大 100MB
   - 使用 Large File Storage (LFS)：最大 2GB（需要付费）

2. **API 速率限制**：
   - 未认证：每小时 60 次请求
   - 认证：每小时 5000 次请求
   - 文件写入操作会消耗较多配额

3. **性能问题**：
   - 不适合高频读写
   - 下载速度受 GitHub 服务器位置影响
   - 没有 CDN 加速（除非使用 GitHub Pages）

4. **存储成本**：
   - 私有仓库：无限存储，但大文件会占用大量 Git 历史
   - 公共仓库：免费，但文件公开可见

---

## 配置项

### 配置文件位置

配置文件默认保存在用户主目录下：

| 操作系统 | 路径 |
|---------|------|
| macOS/Linux | `~/.ghoss/config.yaml` |
| Windows | `%APPDATA%\ghoss\config.yaml` |

也可以通过环境变量 `GHOSS_CONFIG` 指定自定义路径。

### 配置内容

```yaml
github:
  token: "your_github_token"      # GitHub Personal Access Token
  owner: "your_username"          # GitHub 用户名
  repo: "your_repo"               # 仓库名称
  branch: "main"                  # 分支名称（默认 main）

storage:
  root_path: "assets"             # 文件存储根目录
  max_file_size: "100MB"          # 最大文件大小限制

cache:
  enabled: true                   # 是否启用本地缓存
  dir: "~/.ghoss/cache"           # 缓存目录
```

### 安全注意事项

1. **文件权限**：配置文件包含敏感 Token，初始化时自动设置 `chmod 600`
   ```bash
   chmod 600 ~/.ghoss/config.yaml
   ```

2. **不要提交到 Git**：`.gitignore` 中已排除配置文件
   ```
   # .gitignore
   .ghoss/config.yaml
   ~/.ghoss/
   ```

3. **Token 权限**：仅需 `repo` 权限（私有仓库）或 `public_repo`（公共仓库）

---

## 依赖库

- `github.com/spf13/cobra` - CLI 框架
- `github.com/spf13/viper` - 配置管理
- `golang.org/x/oauth2` - OAuth 认证
- `github.com/google/go-github/vXX/github` - GitHub API SDK

---

## 开发计划

### Phase 1: 基础功能
- [ ] 项目初始化（go mod）
- [ ] 配置管理模块
- [ ] GitHub API 客户端
- [ ] 文件上传/下载功能
- [ ] CLI 命令实现

### Phase 2: 增强功能
- [ ] 文件列表/删除功能
- [ ] 获取访问链接
- [ ] 本地缓存机制
- [ ] 进度条显示

### Phase 3: 优化
- [ ] 断点续传
- [ ] 批量操作
- [ ] 加密存储（可选）
- [ ] 单元测试

---

## 安全注意事项

1. **Token 安全**：GitHub Token 应存储在本地配置文件，不要提交到版本控制
2. **文件隐私**：使用私有仓库存储敏感文件
3. **访问控制**：合理设置 Token 的权限范围（仅需 `repo` 权限）

---

## 后续扩展方向

1. **加密功能**：上传时对文件进行加密，增加安全性
2. **版本历史**：利用 Git 的历史记录功能追踪文件变更
3. **多仓库支持**：支持管理多个 GitHub 仓库
4. **CDN 集成**：结合 Cloudflare 等 CDN 加速访问
5. **其他存储后端**：支持腾讯云 CloudBase、AWS S3 等
