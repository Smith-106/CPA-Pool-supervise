# OpenCode Go Pool

管理多个 [opencode.ai](https://opencode.ai) 账号的池化工具。定时抓取每个账号的配额和用量，并将有效账号的 API Key 自动同步到 [CLIProxyAPI (CPA)](https://github.com/router-for-me/CLIProxyAPI) 实现多账号负载均衡。

编译产物为单个 Go 二进制文件，内嵌 React 管理界面，无需额外依赖，开箱即用。

## 功能

- **仪表盘**：一屏查看所有账号的 Rolling / Weekly / Monthly 配额使用率及重置倒计时
- **账号管理**：添加、编辑、启用/禁用、删除账号；一键手动触发单账号刷新
- **定时抓取**：按配置间隔（默认 5 分钟）自动抓取全部账号的配额快照和用量记录
- **用量分析**：按账号查看每日费用趋势图（含历史三个月数据）和详细 Token 消耗明细
- **配额超限自动剔除**：为每个账号单独设置 Rolling / Weekly / Monthly 阈值，超限时自动从 CPA 中移除该账号的 API Key，恢复后自动重新加入
- **CPA 自动同步**：账号增删改或状态变化时立即同步，合并更新不影响 CPA 中的其他提供商
- **亮色 / 暗色主题**

## 界面截图

### 仪表盘（Home）

![仪表盘](pic/Home.png)

### 账号管理（Accounts）

![账号管理](pic/Accounts.png)

## 快速开始

**1. 在 CPA 中提前创建对应的 OpenAI 兼容提供商**（必须）

> 同步逻辑依赖提供商在 CPA 侧已存在。详见 [部署文档 → 前提条件](DEPLOY.md#1-在-cpa-中创建-openai-兼容提供商必须提前完成)。

**2. 构建**

需要 Go 1.21+ 和 Node.js 18+。前端产物会嵌入到 Go 二进制中，因此 `go build` / `go run` 前需先构建前端。

```bash
# 1) 构建前端并复制到 embed 目录
cd web
npm ci
npm run build
cd ..
cp -r web/dist internal/frontend/dist   # Windows: xcopy web\dist internal\frontend\dist\ /e /i /y

# 2a) 直接运行（开发用，读取当前目录 config.yaml）
go run ./cmd/server

# 2b) 编译当前平台二进制
go build -o opencode-pool ./cmd/server

# 2c) 交叉编译到 Linux amd64（部署用）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o build/opencode-pool-linux-amd64 ./cmd/server
```

Windows 也可一键完成前端构建 + 测试 + Linux 交叉编译：

```bat
build.bat
:: 产物：build/opencode-pool-linux-amd64
```

**3. 配置**

```bash
cp config.example.yaml config.yaml
# 填写 server.password、cpa.endpoint、cpa.bearer_token
```

**4. 运行**

```bash
mkdir -p data
./opencode-pool                  # 或 ./build/opencode-pool-linux-amd64
# 打开 http://localhost:8080
```

完整部署说明（systemd、Cookie 获取方式、字段含义等）见 [DEPLOY.md](DEPLOY.md)。

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go · chi · SQLite（pure Go，无 CGO） |
| 前端 | React 19 · TypeScript · Vite · TanStack Query · Tailwind CSS v4 · Radix UI · Recharts |
| 部署 | 单二进制，前端通过 `embed.FS` 内嵌 |

## 友情链接

- [LinuxDo](https://linux.do)
