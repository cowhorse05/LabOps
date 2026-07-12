# 简历项目描述

> **目标读者:** 求职者、面试准备者
> **注意:** 以下描述均基于项目真实功能。不虚构用户量、设备数量、并发量或性能数据。

---

## 一句话项目介绍

| 版本 | 内容 |
|------|------|
| 30 字版 | LabOps：自研的轻量级运维平台，通过 Web 界面实时监控和管理多台远程设备。 |
| 50 字版 | LabOps 是一个基于 Go + React 的轻量级运维平台，实现 Server-Agent 实时通信架构，支持设备监控、远程命令执行、AI 辅助运维分析和完全私有化部署。 |
| 100 字版 | LabOps 是一个从零构建的开源运维平台，采用 Go 标准库实现 HTTP 服务和 WebSocket 实时通信，React + Ant Design 构建管理界面。项目实现了完整的设备接入认证（一次性接入码 + 每设备独立凭据）、命令执行安全约束（四层防护）、数据库方言抽象（SQLite/MySQL 双后端）、Docker Compose 生产部署（TLS + 内网隔离 + 健康检查）和 systemd 安全加固。已成功部署在阿里云 Linux 服务器上并通过公网访问。 |

---

## 简历项目描述

### 实习或初级开发岗位版本

**项目名称：** LabOps — 轻量级运维管理平台

**技术栈：** Go、React、TypeScript、Ant Design、MySQL、WebSocket、Docker、Nginx、Linux

**个人职责：**
- 独立完成项目从需求分析、架构设计到编码实现的全流程开发
- 使用 Go 标准库 `net/http` 实现 REST API（34 个端点）和 WebSocket 服务端
- 使用 React + TypeScript + Ant Design 构建了 12 个页面的管理界面
- 设计并实现了 Server-Agent 通信协议，支持设备注册、心跳上报、命令下发
- 编写 Docker Compose 配置实现一键部署（MySQL + Server + Nginx）
- 完成项目在阿里云 Linux 服务器上的生产部署

**项目成果：**
- 实现完整的 Agent-Server-Web 实时运维闭环，无模拟数据
- 支持 SQLite / MySQL 双数据库后端，通过方言抽象统一接口
- 实现一次性接入码 + 每设备独立凭据的安全认证机制
- 配置 Nginx HTTPS 反向代理和 Let's Encrypt 证书自动续期
- 项目已开源（MIT 许可证）并部署在公网服务器上

---

### 后端开发岗位版本

**项目名称：** LabOps — Server-Agent 架构的运维平台

**技术栈：** Go 1.25、gorilla/websocket、MySQL 8.0、Session Cookie + CSRF、systemd

**个人职责：**
- 使用 Go 标准库 `net/http` 设计 RESTful API，支持 Go 1.22+ 模式路由，共 34 个端点
- 设计 WebSocket 通信协议（JSON 信封 + 6 种消息类型），实现 Agent 实时双向通信
- 实现 Session Cookie + CSRF 双重提交模式的认证方案（替代 JWT 以获得服务端可撤销能力）
- 设计数据库方言抽象层（Dialect 接口），使项目同时支持 SQLite 和 MySQL 而不需维护两套 SQL
- 实现 RBAC 权限模型（管理员/操作员/观察者）和四层命令执行安全约束
- 实现 AI Ops 分析引擎：规则评分（0-100）+ LLM 增强（OpenAI/Anthropic API）+ 自动执行只读命令
- 编写 50+ 个 Go 测试函数，覆盖 API 处理器、WebSocket、数据库 CRUD、并发安全性

**项目成果：**
- 设计了一次性接入码 + 每设备 256 位独立密钥的设备认证机制
- 使用 bcrypt（cost 12）+ AES-256-GCM + 常量时间比较构建多层安全方案
- 实现设备接入、命令执行、审计日志的完整事务处理
- 通过 Go race detector 验证 WebSocket Hub 的并发安全性

---

### DevOps / 运维开发岗位版本

**项目名称：** LabOps — 多环境部署的运维管理平台

**技术栈：** Docker、Docker Compose、Nginx、Let's Encrypt、systemd、MySQL、GitHub Actions

**个人职责：**
- 设计 Docker Compose 三层网络隔离架构（backend/egress/edge），数据库和 API 不暴露公网
- 配置 Nginx 反向代理（HTTPS 终止 + WebSocket 升级 + SPA 路由回退）
- 编写 Dockerfile 实现多阶段构建（Go 编译 → Alpine 运行时），镜像体积约 20MB
- 配置 Let's Encrypt SSL 证书自动申请和续期（certbot standalone + renew timer）
- 编写 Agent 的 hardened systemd 服务单元（10 项安全限制：NoNewPrivileges、ProtectSystem=strict 等）
- 实现 MySQL 自动备份脚本（mysqldump + gzip），7 天日备份 + 4 周周备份保留策略
- 编写 GitHub Actions CI 流水线：Go vet + test -race + TypeScript check + Docker build 验证

**项目成果：**
- 实现一条 `docker compose up -d` 命令完成完整部署
- 设计数据持久化方案（Docker Volume 命名卷 + 宿主机 bind mount），明确区分数据安全操作
- 完成从本地开发 → Docker 测试 → 阿里云生产部署的全流程落地
- 编写完整的服务器部署教程（13 章），覆盖从购买服务器到 HTTPS 上线的每一个步骤

---

## 项目经历表述（STAR 风格写作）

### 表述 1：Server-Agent 通信架构

> 面对 NAT 和防火墙后设备难以管理的场景，我设计并实现了基于 WebSocket 的 Server-Agent 通信架构。Agent 主动连接中央服务建立持久双向通道，每 10 秒上报 CPU/内存/磁盘使用率，服务端通过 WebSocket 推送命令。这种方式不需要中央服务能访问 Agent（出站连接即可），解决了跨网络环境的设备管理问题。

### 表述 2：数据库双后端设计

> 为解决开发环境零配置和生产环境可靠性的矛盾，我设计了 Dialect 接口将 SQL 差异抽象为统一接口。Schema 定义使用逻辑列类型（如 CT_String32），由各方言映射为对应的物理类型（MySQL: VARCHAR(32)，SQLite: TEXT）。切换数据库只需修改一个环境变量，12 张表在两种数据库下行为一致。

### 表述 3：Docker 生产部署

> 为实现安全的一键部署，我使用 Docker Compose 定义了三个服务和三层网络隔离。Backend 网络设为 internal，MySQL 和 Server 的端口完全不暴露公网。TLS 证书通过宿主机 bind mount 以只读方式挂载。每个服务配置了健康检查和依赖顺序，确保服务按正确顺序启动。

### 表述 4：命令执行安全约束

> 针对 Web 界面执行远程命令的安全风险，我设计了四层防护：确认键在服务端校验而非前端校验；三级 RBAC 权限控制；15+ 种正则模式自动拦截危险命令；256KB 输出截断和 300 秒超时限制。LLM 推荐的不安全命令在 dispatch 之前就被过滤。

---

## 面试自我介绍

### 30 秒版本

> 我独立开发了一个叫 LabOps 的开源运维平台，技术栈是 Go + React。核心功能是通过 Web 界面实时监控和管理多台远程设备——Agent 通过 WebSocket 上报系统指标和心跳，管理员可以在 Web 界面下发命令并查看执行结果。项目已经部署在我的阿里云服务器上，可以通过公网访问。

### 1 分钟版本

> 我独立开发了一个从零到一的完整开源项目叫 LabOps，是一个轻量级的运维管理平台。核心架构是 Server-Agent 模式：Go 写的中央服务部署在云服务器上，用 React + TypeScript 构建了 12 个页面的管理界面，Agent 是用 Go 写的单文件静态二进制，安装在被管理设备上。
>
> 技术上有几个亮点：一是 Agent 通过 WebSocket 主动连接服务端，不需要公网 IP，解决了 NAT 和防火墙后的设备管理问题。二是设备安全认证——用一次性接入码换取每设备独立的 256 位密钥，而不是所有设备共享一个密码。三是数据库层做了方言抽象，能同时支持 SQLite 和 MySQL 而不需要维护两套代码。四是 Docker 部署方案用了三层网络隔离，数据库和 API 端口完全不暴露公网。
>
> 项目已开源在 GitHub 上，并部署在阿里云 Linux 服务器上通过 HTTPS 公网访问。

### 3 分钟版本

> （在前 1 分钟基础上补充）
>
> 在安全方面，我实现了一套比较完整的多层防护。用户认证用的是 Session Cookie 加 CSRF 双重提交模式而放弃了 JWT——因为 Session 可以在服务端随时撤销，用户改密后所有旧会话立刻失效。命令执行有四层约束：前端确认键在校验后端、RBAC 权限控制、15 多种危险命令的正则拦截、256KB 输出截断和 300 秒超时。LLM API Key 用 AES-256-GCM 加密存储。
>
> 在部署方面，我对 Docker 生产环境做了仔细的安全配置。Compose 文件定义了三层网络：内部网络让 MySQL 和 Server 不暴露端口、出站网络让 Server 能访问 LLM API、边缘网络只让 Nginx 对外。Agent 的 systemd 服务配置了 10 项安全限制，包括禁止提权、文件系统只读、独立的临时目录等。
>
> 项目里还有一些我比较满意的工程实践：前后端分离的清晰架构、数据库迁移的版本化管理、GitHub Actions 的 CI 自动测试。整个过程让我从一个想法开始，经历了需求分析 → 架构设计 → 编码 → 测试 → 部署上线的完整周期，对全栈开发、Linux 运维、Docker 容器化都有了比较扎实的理解。
