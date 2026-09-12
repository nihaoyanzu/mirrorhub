# aimirror 2.0 架构设计文档

> AI 时代的下载镜像加速平台 —— 多平台代理、智能缓存、限速与优先级调度

## 已定决策（2026-09-09）

| 决策项 | 结论 |
|--------|------|
| 后端 | **Go**（chi），全部重构 |
| 前端 | Vue 3 + TypeScript + Tailwind CSS + Yarn + Vite |
| 部署 | **Nginx 拆开**：静态前端 / 管理 API / 代理流量分流 |
| 插件 | **不做插件化、不做热加载插件** |
| 平台扩展 | 内置 `handlers` + 配置规则，代码内扩展 |
| 限速 | 支持按平台拉取限速 |
| 调度 | **用户主动拉取优先**，压低后台/限时预拉取 |
| 本阶段范围 | **仅 PyPI**；Docker / HuggingFace / CRAN 延后 |
| 配置 | **`.env` 主要只要 `DATABASE_URL`**；上游/限速等进 **PostgreSQL**，管理页维护 |
| 鉴权 | 简单登录（默认 `admin` / `admin`），会话 Token，无 `AIMIRROR_ADMIN_TOKEN` |

---

## 1. 目标与原则

### 1.1 目标

- 统一加速 HTTP 下载（本阶段 PyPI；后续 Docker / HF / CRAN）
- 并行分片 + 本地缓存
- 平台级限速与优先级调度（P0 交互 > P2 预拉取）

### 1.2 原则

| 原则 | 说明 |
|------|------|
| **配置驱动** | 平台规则、限速、调度权重以配置为主 |
| **核心统一** | 下载、缓存、限速、调度由核心实现，handler 只处理协议差异 |
| **前后端分离** | Nginx 统一入口，前端与后端独立发布 |
| **可观测** | 限速命中、队列等待、优先级抢占可监控 |
| **简单可维护** | 不做插件市场/热插拔 |

---

## 2. 总体架构

```
Nginx: / → frontend; /api/v1 → admin; mirror host → proxy
                │
         Go backend (双端口)
         ├── proxy  :8081  数据面
         └── admin  :8082  管理面
                │
         Router → handlers/pypi
                │
         Scheduler → RateLimiter → Downloader → Cache → Upstream
```

---

## 3. 后端模块（Go）

```
backend/
├── cmd/aimirror/main.go
├── internal/
│   ├── config/
│   ├── router/
│   ├── cache/
│   ├── downloader/
│   ├── ratelimit/
│   ├── scheduler/
│   ├── handlers/pypi/
│   ├── proxy/
│   ├── api/
│   └── metrics/
├── configs/config.yaml
└── go.mod
```

| 模块 | 职责 |
|------|------|
| Router | 按 path 选平台与策略 |
| Handler | PyPI 链接改写等协议差异 |
| Downloader | Range 分片、重试、断点续传 |
| Cache | 命中、LRU、TTL |
| RateLimiter | 按平台带宽/并发 |
| Scheduler | P0/P2 排队与抢占 |

---

## 4. 限速与调度

- 每平台：`bandwidth_mbps`、`max_concurrent`、`max_connections`
- P0 Interactive / P1 Resume / P2 Prefetch
- P2 仅用 `idle_quota_ratio`；有 P0 时 pause P2（保留断点）
- `small_file_boost`：索引小文件插队

预拉取来源：管理 API 手动入队 + 配置时间窗 URL 列表。

---

## 5. PyPI（本阶段）

| 路径 | 策略 |
|------|------|
| `/simple` | proxy + 短 TTL + URL 改写 |
| `/packages/*.metadata` | proxy |
| `/packages` | parallel + 缓存 |

---

## 6. Web 与 Nginx

- 管理台：仪表盘、PyPI 限速、队列、缓存、预拉取
- 鉴权：管理台账号密码登录（默认 admin/admin）
- Nginx：admin 静态+/api；mirror 反代 proxy

---

## 7. 延后

Docker / HuggingFace / CRAN / NVIDIA / PyTorch 专用规则。

---

*文档版本: 2.1*  
*最后更新: 2026-09-09*
