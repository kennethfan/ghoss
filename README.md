# GHoss - GitHub Object Storage Tool

使用 GitHub 仓库作为对象存储的命令行工具。

## ⚠️ 重要提示

GitHub **不是**为对象存储设计的。本工具仅适用于：
- 个人项目/学习用途
- 低频访问的文件存储
- 小文件分发

**不适用于**生产环境的高频访问场景。

## 特性

- 📤 上传文件到 GitHub 仓库
- 📥 从 GitHub 仓库下载文件
- 📋 列出仓库中的文件（支持长格式、排序、递归、JSON 输出）
- 🗑️ 删除仓库中的文件（支持递归删除目录）
- ✂️ 移动/重命名文件和目录
- 🔗 生成文件公开访问链接
- 💾 本地配置管理
- 📁 支持递归上传目录

## 安装

### 从源码编译

```bash
# 克隆仓库
git clone https://github.com/your-username/ghoss.git
cd ghoss

# 安装依赖
go mod tidy

# 编译
go build -o ghoss cmd/ghoss/main.go

# 或者安装到 GOPATH/bin
go install github.com/ghoss/cmd/ghoss@latest
```

### 配置 PATH

```bash
# macOS/Linux
export PATH=$PATH:$(pwd)

# 或者移动到系统 PATH
sudo cp ghoss /usr/local/bin/
```

## 快速开始

### 1. 初始化配置

```bash
./ghoss init
```

按提示输入：
- GitHub Personal Access Token
- 仓库所有者（用户名或组织名）
- 仓库名称
- 分支名称（默认 main）

配置文件保存在 `~/.ghoss/config.yaml`

### 2. 创建 GitHub Token

1. 访问 [GitHub Settings > Developer settings > Personal access tokens](https://github.com/settings/tokens)
2. 点击 **Generate new token (classic)**
3. 勾选 `repo` 权限（私有仓库）或 `public_repo`（公共仓库）
4. 复制 Token 并保存到安全位置

### 3. 上传文件

```bash
# 上传单个文件
./ghoss upload ./photo.jpg

# 上传到指定路径
./ghoss upload ./document.pdf --path docs/report.pdf

# 递归上传目录
./ghoss upload ./images/ --recursive
```

### 4. 下载文件

```bash
# 下载到当前目录
./ghoss download photos/image.jpg

# 下载到指定路径
./ghoss download photos/image.jpg --output ./local/image.jpg
```

### 5. 列出文件

```bash
# 列出根目录
./ghoss ls

# 列出子目录
./ghoss ls photos/

# 长格式（带大小和时间）
./ghoss ls -l photos/

# 按时间排序
./ghoss ls -t

# 递归列出子目录
./ghoss ls -R

# JSON 格式输出
./ghoss ls --json
```

### 6. 移动/重命名文件

```bash
# 重命名文件
./ghoss mv docs/report.pdf docs/report-2026.pdf

# 移动到目录
./ghoss mv image.png images/

# 自定义提交信息
./ghoss mv src.txt dst.txt -m "move file"

# 移动目录（自动递归）
./ghoss mv photos/vacation/ backup/2026/
```

### 7. 获取访问链接

```bash
./ghoss url photos/image.jpg
```

### 8. 删除文件

```bash
# 删除单个文件
./ghoss rm photos/image.jpg

# 递归删除目录
./ghoss rm -r old-photos/
```

## 命令参考

### `ghoss init`

初始化配置文件。

```bash
./ghoss init [config-path]
```

### `ghoss upload`

上传文件到 GitHub 仓库。

```bash
./ghoss upload <file-path> [flags]
```

**Flags:**
- `-p, --path string`: 目标路径（在仓库中）
- `-m, --message string`: Git 提交信息
- `-r, --recursive`: 递归上传目录
- `-v, --verbose`: 详细输出

**示例:**
```bash
./ghoss upload ./file.txt
./ghoss upload ./file.txt --path backup/file.txt
./ghoss upload ./folder/ --recursive
```

### `ghoss download`

从 GitHub 仓库下载文件。

```bash
./ghoss download <file-path> [flags]
```

**Flags:**
- `-o, --output string`: 输出文件路径
- `-v, --verbose`: 详细输出

**示例:**
```bash
./ghoss download photos/image.jpg
./ghoss download photos/image.jpg -o ./local/image.jpg
```

### `ghoss ls`

列出仓库中的文件。

```bash
./ghoss ls [path] [flags]
```

**Flags:**
- `-l, --long`: 长格式（显示权限、大小、修改时间）
- `-h, --human-readable`: 人类可读文件大小
- `--sort name|time|size`: 排序字段（默认 name）
- `-t, --sort-by-time`: 按修改时间排序
- `-S, --sort-by-size`: 按文件大小排序
- `-r, --reverse`: 反向排序
- `-R, --recursive`: 递归列出子目录
- `--json`: JSON 格式输出

**示例:**
```bash
./ghoss ls
./ghoss ls photos/
./ghoss ls -l
./ghoss ls -t -r
./ghoss ls -R --json
```

### `ghoss mv`

移动/重命名文件或目录。目录移动自动递归。

```bash
./ghoss mv <source> <destination> [flags]
```

**Flags:**
- `-m, --message string`: Git 提交信息

**示例:**
```bash
./ghoss mv file.txt renamed.txt
./ghoss mv file.txt images/
./ghoss mv old-dir/ new-dir/
./ghoss mv src dst -m "my commit message"
```

### `ghoss rm`

删除仓库中的文件。

```bash
./ghoss rm <file-path> [flags]
```

**Flags:**
- `-r, --recursive`: 递归删除目录

**示例:**
```bash
./ghoss rm photos/old-image.jpg
./ghoss rm -r old-photos/
```

### `ghoss url`

生成文件的公开访问链接。

```bash
./ghoss url <file-path>
```

**示例:**
```bash
./ghoss url photos/image.jpg
# 输出: https://raw.githubusercontent.com/username/repo/main/photos/image.jpg
```

## 配置

配置文件位于 `~/.ghoss/config.yaml`：

```yaml
github:
  token: "ghp_xxxxxxxxxxxx"    # GitHub Token
  owner: "username"             # 仓库所有者
  repo: "repo-name"             # 仓库名称
  branch: "main"                # 分支

storage:
  root_path: "assets"           # 存储根目录
  max_file_size: "100MB"        # 最大文件大小

cache:
  enabled: true                 # 启用缓存
  dir: "~/.ghoss/cache"         # 缓存目录
```

### 环境变量

- `GHOSS_CONFIG`: 自定义配置文件路径
- `GHOSS_CONFIG_DIR`: 自定义配置目录

## 限制

### GitHub 限制

| 限制项 | 值 |
|--------|-----|
| 单文件大小 | 100MB |
| API 速率限制（已认证） | 5,000 次/小时 |
| API 速率限制（未认证） | 60 次/小时 |

### 本工具限制

- 不支持大文件（>100MB）
- 不适合高频访问
- 文件历史会占用 Git 仓库空间
- 下载速度受 GitHub 服务器位置影响

## 项目结构

```
ghoss/
├── cmd/
│   └── ghoss/
│       └── main.go          # CLI 入口（Cobra 命令）
├── internal/
│   ├── github/
│   │   └── client.go        # GitHub API 客户端
│   ├── storage/
│   │   └── storage.go       # 本地文件操作
│   ├── lister/
│   │   └── lister.go        # ls 输出引擎
│   └── config/
│       └── config.go        # 配置管理
├── pkg/
│   └── models/
│       └── file.go          # 数据模型
├── bin/                     # 编译产物
├── go.mod
├── README.md
└── .gitignore
```

## 安全注意事项

1. **保护 Token**：不要将 Token 提交到版本控制
2. **使用私有仓库**：存储敏感文件时使用私有仓库
3. **最小权限**：Token 只需授予 `repo` 或 `public_repo` 权限
4. **定期轮换**：定期更换 Token

## 后续计划

- [ ] 支持加密存储
- [ ] 实现本地缓存机制
- [ ] 断点续传
- [ ] 批量操作
- [ ] 支持多仓库
- [ ] Web UI

## 许可证

APACE 2.0

## 贡献

欢迎提交 Issue 和 Pull Request！

---

**免责声明**：本工具仅供学习和个人项目使用。GitHub 不是为对象存储设计的，不建议在生产环境中使用。
