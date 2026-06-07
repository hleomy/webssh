# WebSSH

> Xshell 风格的 Web SSH 客户端，支持终端、SFTP、端口转发、服务器分组、密钥管理、深色/浅色主题。

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)
![License](https://img.shields.io/badge/license-MIT-green)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)

一个用 **Go + React** 构建的轻量级 Web SSH 工具，自带 SQLite 数据库（**无需 GCC**），单镜像部署，支持 1Panel 编排一键上线。

## 特性

- **终端** - 多标签页，基于 xterm.js + WebSocket，支持密码 / 私钥认证、保活、自适应窗口
- **SFTP** - 文件浏览、上传、下载、编辑、重命名、删除、新建目录 / 文件、权限查看
- **端口转发** - 本地转发 + 远程转发，启动 / 停止 / 删除
- **服务器管理** - 分组、收藏、标签、CRUD
- **认证** - 首次进入自动注册管理员账号，bcrypt 密码哈希 + JWT Token
- **凭据加密** - 密码 / 私钥 AES-256-GCM 加密存储
- **主题** - 深色 / 浅色，字体大小可调，多字体选择，光标闪烁开关
- **零依赖部署** - 纯 Go SQLite 驱动，多阶段 Docker 镜像 < 50MB

## 界面预览

打开浏览器访问 `http://<server>:6970`，首次进入自动跳到注册页创建管理员账号。

## 快速开始（Docker）

```bash
# 1. 克隆
git clone https://github.com/hleomy/webssh.git
cd webssh

# 2. 构建并启动
docker compose build
docker compose up -d

# 3. 访问
open http://localhost:6970
```

数据持久化在 `./data/webssh.db`，日志在 `./data/logs/webssh.log`。

## 1Panel 部署

1Panel 编排默认只 `docker pull` 不 `build`，本项目通过 `build:` + `image: webssh:latest` 模式，让你在服务器上先 `docker compose build` 一次，之后编排会复用本地镜像。

```bash
# 在 1Panel 服务器
cd /opt/webssh
docker compose build      # 首次必须先做这一步
```

然后在 1Panel 控制台：**容器 → 编排 → 创建编排** → 路径选 `/opt/webssh` → 部署。

> 大陆服务器建议先在 `/etc/docker/daemon.json` 配置镜像加速：
> ```json
> { "registry-mirrors": ["https://mirror.ccs.tencentyun.com"] }
> ```
> 然后 `systemctl restart docker`。

## 端口

| 端口 | 用途 |
|------|------|
| 6970 | Web 界面、REST API、WebSocket |

## 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `WEBSSH_SERVER_HOST` | `0.0.0.0` | 监听地址 |
| `WEBSSH_SERVER_PORT` | `6970` | 监听端口 |
| `WEBSSH_SERVER_MODE` | `release` | Gin 模式（`debug` / `release`） |
| `WEBSSH_JWT_SECRET` | `please-change-me...` | **生产务必修改** |
| `WEBSSH_JWT_EXPIRE_HOURS` | `168` | Token 有效期（小时） |
| `WEBSSH_DATA_DIR` | `./data` | 数据目录 |
| `WEBSSH_SSH_CONNECTION_TIMEOUT` | `15` | SSH 连接超时（秒） |
| `WEBSSH_SSH_KEEP_ALIVE_INTERVAL` | `30` | SSH 保活间隔（秒） |
| `TZ` | `Asia/Shanghai` | 时区 |

## 本地开发

### 后端
```bash
go mod tidy
go run ./cmd/server
# 默认 http://localhost:6970
```

### 前端
```bash
cd web
npm install
npm run dev
# 默认 http://localhost:5173（自动代理后端 6970）
```

## 架构

```
┌─────────────┐     HTTPS/WS     ┌──────────────┐     SSH      ┌─────────┐
│   Browser   │ ◄──────────────► │  Go Backend  │ ◄──────────► │  Server │
│  React +    │   /api /ws/ssh   │  Gin + WS    │   TCP 22     │         │
│  xterm.js   │                  │  golang.org/ │              └─────────┘
└─────────────┘                  │  x/crypto    │     SFTP      ┌─────────┐
                                 │  pkg/sftp    │ ◄──────────► │  Server │
                                 │  SQLite      │              └─────────┘
                                 └──────────────┘
```

## 目录结构

```
webssh/
├── cmd/server/              # Go 入口
├── internal/
│   ├── auth/                # JWT + AES-256-GCM
│   ├── config/              # 配置 + 数据库
│   ├── handler/             # HTTP 处理器
│   ├── middleware/          # JWT 中间件
│   ├── model/               # 数据模型 + 迁移
│   ├── sftpserver/          # SFTP 服务
│   ├── sshclient/           # SSH 客户端 / 端口转发
│   └── wsbridge/            # WebSocket ↔ SSH 桥
├── web/                     # React 前端
│   ├── src/
│   └── public/
├── deployments/             # 配置示例 + entrypoint
├── Dockerfile               # 多阶段构建
├── docker-compose.yml       # 1Panel 编排
├── go.mod / go.sum
├── README.md
└── LICENSE
```

## 安全提示

- 生产环境**必须**修改 `WEBSSH_JWT_SECRET` 为随机长字符串
- 建议通过反向代理（Nginx / Caddy）启用 HTTPS
- 凭据使用 AES-256-GCM 加密，但密钥即 `JWT_SECRET`，请妥善保管
- WebSocket 当前允许所有来源，生产可在 `internal/wsbridge/bridge.go` 中收紧

## 故障排查

### 容器一直重启，日志出现 `permission denied` / `unable to open database file`
**根因**：`./data` 挂载到容器后，宿主机目录属主（root）覆盖了容器内 chown，uid=1000 的 webssh 用户写不进去。

**修复**（任选一种）：

a) 本项目用 `deployments/entrypoint.sh` 自动 `chown -R 1000:1000 /app/data`：

b) 手动修复：
```bash
docker compose down
sudo chown -R 1000:1000 /opt/webssh/data
docker compose up -d
```

### 1Panel 报 `pull access denied for webssh`
1Panel 默认 `docker pull webssh:latest`，因本地不存在该镜像。  
**解决**：在服务器先 `cd /opt/webssh && docker compose build`。

### `failed to solve: ... go.mod requires go >= 1.23.0`
Dockerfile 固定 `golang:1.22-alpine` + `CGO_ENABLED=0`，并使用纯 Go SQLite 驱动。  
如仍出现，请确认 `go.mod` / `go.sum` 未被改过。

### 前端 404
镜像构建需要 Node 阶段；如直接用 `go run`，需先 `cd web && npm run build` 让 `web/dist` 存在。

## 路线图

- [ ] Web 终端命令补全
- [ ] 多用户 / 权限管理
- [ ] 审计日志
- [ ] 主机监控指标
- [ ] 文件在线预览（图片、代码高亮）

## 贡献

欢迎 PR 和 Issue。

## 许可证

[MIT](LICENSE)
