# LabOps 文档索引

欢迎阅读 LabOps 项目文档。本文档帮助你快速找到所需内容。

---

## 文档目录

```
docs/
│
├── README.md                          # ← 你在这里
│
├── 📁 project/                        # 项目级文档
│   ├── overview.md                    #   项目概述：解决什么问题、功能概览
│   ├── architecture.md                #   系统架构：数据库、协议、认证、AI Ops
│   └── project-highlights.md          #   技术亮点：每个亮点的代码路径和面试表述
│
├── 📁 deployment/                     # 部署文档
│   ├── overview.md                    #   部署方式概览与对比表
│   ├── server-deployment.md           #   ★ 服务器部署教程（新手友好，从零到上线）
│   ├── docker-compose.md              #   Docker Compose 部署详解
│   ├── native-linux.md                #   原生 Linux 部署（systemd）
│   ├── nginx-and-https.md             #   Nginx 反向代理与 HTTPS 配置
│   └── agent-deployment.md            #   Agent 部署详解
│
├── 📁 user-guide/                     # 用户手册
│   ├── quick-start.md                 #   快速开始：登录、改密、添加第一台设备
│   ├── device-management.md           #   设备管理：列表、详情、实时指标、分组
│   ├── task-management.md             #   任务管理：临时命令、模板、批量下发
│   └── aiops-usage.md                 #   AI Ops：健康报告、LLM 配置、推荐命令
│
├── 📁 operations/                     # 运维文档
│   ├── data-storage.md                #   数据持久化：每条数据的存储位置和备份策略
│   ├── backup-restore.md              #   备份与恢复：手动/自动备份、恢复流程
│   ├── migration.md                   #   服务器迁移：从旧服务器迁移到新服务器
│   └── upgrade-rollback.md            #   升级与回滚：版本更新、回滚策略
│
├── 📁 troubleshooting/                # 故障排查
│   ├── ssh.md                         #   SSH 连接问题
│   ├── nginx.md                       #   Nginx 配置与运行问题
│   ├── docker.md                      #   Docker 容器问题
│   ├── dns-and-https.md               #   DNS 和 HTTPS 证书问题
│   └── agent.md                       #   Agent 连接与运行问题
│
├── 📁 career/                         # 求职面试
│   ├── resume-project.md              #   简历项目描述（三个岗位版本）
│   ├── interview-questions.md         #   面试问题与参考回答（30+ 题）
│   └── star-stories.md                #   STAR 案例（8 个真实场景）
│
├── source-code-guide.md               # 源码阅读指南（教科书风格，15 章）
├── master-plan.md                     # 项目总体规划（SSOT）
├── roadmap.md                         # 版本路线图
├── research.md                        # 竞品分析
├── dev-log.md                         # 开发日志
├── log.md                             # 变更日志
├── report.md                          # 项目报告
├── security.md                        # 安全模型
└── features/file-distribution/        # v0.3 文件分发设计规范
```

---

## 按角色推荐阅读顺序

### 我是第一次接触这个项目的新用户

1. [项目概述](project/overview.md) — 了解 LabOps 是什么
2. [服务器部署教程](deployment/server-deployment.md) — 把项目部署到你的服务器
3. [快速开始](user-guide/quick-start.md) — 登录并添加第一台设备

### 我需要在服务器上部署这个项目

1. [部署方式概览](deployment/overview.md) — 选择适合你的部署方式
2. [服务器部署教程](deployment/server-deployment.md) — 完整部署流程（推荐）
3. [Agent 部署详解](deployment/agent-deployment.md) — 在目标机器上安装 Agent
4. [Nginx 与 HTTPS](deployment/nginx-and-https.md) — 配置反向代理和证书
5. [备份与恢复](operations/backup-restore.md) — 配置自动备份

### 我需要运维已部署的项目

1. [数据持久化](operations/data-storage.md) — 理解数据存在哪里
2. [备份与恢复](operations/backup-restore.md) — 备份和恢复操作
3. [升级与回滚](operations/upgrade-rollback.md) — 版本升级流程
4. [服务器迁移](operations/migration.md) — 更换服务器
5. [故障排查](troubleshooting/) — 遇到问题找这里

### 我是开发者，想理解项目内部

1. [系统架构](project/architecture.md) — 完整的技术架构文档
2. [源码阅读指南](source-code-guide.md) — 教科书式的代码阅读路线
3. [项目亮点](project/project-highlights.md) — 技术实现细节

### 我准备面试，需要复习项目

1. [简历项目描述](career/resume-project.md) — 简历和自我介绍模板
2. [面试问题](career/interview-questions.md) — 30+ 题目与参考回答
3. [STAR 案例](career/star-stories.md) — 8 个真实项目经历故事
4. [项目亮点](project/project-highlights.md) — 技术深度的证据

---

## 文档约定

- 所有命令、路径、端口均来自项目实际代码和配置文件
- 无法从代码确认的信息会标注「当前无法从代码仓库确认」
- 部署命令同时提供 dnf（Alibaba Cloud Linux/Rocky Linux）和 apt（Ubuntu/Debian）版本
- 每份文档开头标注目标读者和前置阅读要求
