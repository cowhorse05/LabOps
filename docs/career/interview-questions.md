# 面试问题与参考回答

> **目标读者:** 面试准备者
> **说明:** 所有参考回答基于项目真实实现。问题覆盖架构、部署、安全、排错等方面。

---

## 项目概述类

### Q1：用一句话介绍你的项目

**参考回答：** LabOps 是一个开源运维管理平台，通过 Go 服务端 + React 管理界面 + Go Agent 的架构，让用户可以在 Web 界面上实时监控和管理多台远程设备。

### Q2：这个项目解决了什么问题？

**参考回答：** 它解决了三个核心问题。第一，多台设备分散在不同网络环境中（云服务器、家庭网络、公司内网），传统方式需要逐个 SSH 登录管理，LabOps 通过 Agent 主动连接中央服务实现了统一管理。第二，运维操作需要命令行技能，LabOps 提供了图形化界面，降低了使用门槛。第三，第三方 SaaS 运维平台意味着数据在别人服务器上，LabOps 支持完全私有化部署，数据留在用户自己的服务器上。

### Q3：为什么选择当前的技术栈？

**参考回答：** Go 用于后端是因为它的并发模型（goroutine + channel）很适合处理 WebSocket 连接管理，标准库 net/http 足够强大不需要外部框架，静态编译让 Agent 部署非常简单（一个二进制文件复制过去就行）。React + TypeScript 选择是基于生态和类型安全。Ant Design 提供成熟的企业级组件库，中文支持好。SQLite 和 MySQL 的选择是为了覆盖开发零配置和生产可靠性两种场景。

---

## 架构设计类

### Q4：Server 和 Agent 之间怎么通信的？

**参考回答：** 通过 WebSocket 建立持久的双向连接。Agent 主动连接服务端（出站连接，不需要公网 IP），连接后发送注册消息（设备信息），然后每 10 秒发送一次心跳（CPU、内存、磁盘使用率）。服务端可以随时通过 WebSocket 推送命令给 Agent，Agent 执行后返回 stdout、stderr、退出码和耗时。

### Q5：为什么用 WebSocket 而不是 HTTP 轮询？

**参考回答：** 两个原因。第一，服务端需要能随时向 Agent 推送命令，HTTP 是请求-响应模式做不到服务端主动推送。第二，Agent 需要持续上报心跳，HTTP 轮询开销太大（每 10 秒一个 HTTP 请求 vs 一个 WebSocket 帧）。WebSocket 提供低延迟、低开销的双向通信。

### Q6：Agent 怎么处理重连？

**参考回答：** Agent 使用指数退避重连策略。连接断开后，等待 1 秒 → 2 秒 → 4 秒 → ... → 最大 60 秒。连接成功后重置为 1 秒。重连后 Agent 重新发送注册消息，服务端会自动下发之前待处理的任务。

### Q7：数据库为什么支持两种？

**参考回答：** 开发时用 SQLite——零配置，一个文件，clone 代码就能跑。生产环境用 MySQL——更可靠的并发处理、更好的备份工具链。切换只需要改一个环境变量，因为数据库访问层用了方言抽象（Dialect Pattern），将 SQL 差异封装在接口实现中。

---

## 安全类

### Q8：用户认证是怎么做的？为什么不用 JWT？

**参考回答：** 用的是 Session Cookie 而不是 JWT。核心原因是 JWT 是无状态的，服务端没办法主动让它失效——用户改密码后旧的 JWT 还是能用的。Session Cookie 是服务端维护的，可以随时删除。考虑到 LabOps 是单实例部署（不需要分布式会话共享），Session 方案更适合。

### Q9：CSRF 攻击怎么防护的？

**参考回答：** 使用双重提交 Cookie 模式。登录时服务端生成两个 256 位随机 Token——Session Token（HttpOnly Cookie，JS 读不到）和 CSRF Token（普通 Cookie，JS 能读到）。所有状态变更请求（POST/PUT/DELETE）需要在 HTTP Header 中带上 CSRF Token，服务端验证 Cookie 中的 CSRF Token 和 Header 中的一致，且和数据库中存的哈希匹配。

### Q10：Agent 怎么认证的？如果密钥泄露了怎么办？

**参考回答：** 每台设备有独立的 256 位密钥，不是所有设备共享一个密码。密钥通过一次性接入码（有时效性）换取。如果某台设备的密钥泄露了，管理员在 Web 界面点击吊销，那台设备的密钥立刻失效，而且不影响其他设备。所以即使一台设备被攻破，攻击范围也只限于那一台。

### Q11：LLM API Key 怎么存储的？

**参考回答：** 用 AES-256-GCM 加密后存储在数据库中。加密密钥是部署时设置的 `LABOPS_ENCRYPTION_KEY`，不在代码中，不提交到 Git。即使数据库文件泄露，没有加密密钥也无法解密 API Key。

---

## 部署运维类

### Q12：Docker 在项目中的作用是什么？

**参考回答：** 三个作用。一是环境一致性——开发、测试、生产用同样的镜像。二是服务编排——一条命令启动 MySQL + Server + Nginx 三个服务，处理好启动顺序和网络。三是隔离——容器之间用内部网络通信，数据库端口不暴露到宿主机。

### Q13：Nginx 在项目中做了什么？

**参考回答：** 四件事。一是 HTTPS 终止——处理 TLS 加解密，Go 服务端不需要处理证书。二是反向代理——把外部请求转发到内部的 Go 服务端。三是 WebSocket 升级——配置 `Upgrade` 和 `Connection` 头让 Agent 的 WebSocket 连接能通过 Nginx。四是静态文件服务——直接返回 React 构建的 HTML/JS/CSS 文件，不需要经过 Go 后端。

### Q14：怎么保证服务挂了能自动恢复？

**参考回答：** 两层保障。Docker 层面：compose.yaml 中配置了 `restart: unless-stopped`，容器异常退出会自动重启。systemd 层面：Agent 的 service 配置了 `Restart=on-failure`，进程崩溃 5 秒后自动重启。每个服务还有健康检查（MySQL 用 mysqladmin ping、Server 用 /api/health、Nginx 用 /healthz），依赖服务健康后才启动下游服务。

### Q15：证书过期了怎么办？

**参考回答：** Let's Encrypt 证书 90 天过期。Certbot 安装时会自动配置 systemd timer 每天检查两次，到期前自动续期，续期后通过 deploy hook 重载 Nginx。正常情况下不需要人工干预。如果续期失败（比如 80 端口被关了），certbot 会发邮件通知。

---

## 数据处理类

### Q16：数据存在哪里？

**参考回答：** Docker Compose 部署时，MySQL 数据存在 Docker 命名卷 `mysql-data` 中（实际在宿主机的 `/var/lib/docker/volumes/` 下）。Agent 凭据存在每台被管理设备的 `/etc/labops-agent/credentials.json`。TLS 证书存在宿主机的 `/etc/letsencrypt/`。备份存在 `/var/backups/labops/`。

### Q17：删除容器数据会丢吗？

**参考回答：** `docker compose down` 不会丢数据（Volume 保留），但 `docker compose down -v` 会丢（-v 删除 Volume）。我在文档中明确标注了哪些命令安全哪些危险。每天有自动备份到宿主机目录，即使 Volume 被误删也有备份可以恢复。

### Q18：怎么备份和恢复？

**参考回答：** 备份用 mysqldump + gzip，每天凌晨 3:15 通过 systemd timer 自动执行。保留 7 天日备份和 4 周周备份。恢复脚本有安全确认机制——必须设置 `LABOPS_CONFIRM_RESTORE=RESTORE` 环境变量才能执行，防止误操作。

---

## 问题排查类

### Q19：Agent 显示离线怎么排查？

**参考回答：** 按顺序检查：① `systemctl status labops-agent` 看服务是否在运行；② `journalctl -u labops-agent` 看有没有连接错误；③ `curl 服务器/api/health` 确认网络可达；④ 检查 Web 界面中设备凭据是否被吊销；⑤ 检查 agent.env 中服务器 URL 是否正确。

### Q20：部署后 502 Bad Gateway 怎么办？

**参考回答：** 502 表示 Nginx 能运行但连不上后端。先 `curl localhost:8080/api/health` 确认 Go 服务端是否正常。如果 8080 也没响应，`docker compose logs server` 看 Server 日志。常见原因是 MySQL 还没就绪——等 MySQL 健康检查通过后，Server 会自动启动。

### Q21：证书申请失败怎么排查？

**参考回答：** 三个关键条件：域名 DNS 已生效（`nslookup 域名` 返回服务器 IP）、80 端口公网可达（在服务器外 `curl http://域名`）、80 端口没被占用。在中国大陆服务器上还要确认完成了 ICP 备案——备案和证书是两个独立问题。

---

## 工程实践类

### Q22：项目怎么管理依赖的？

**参考回答：** Go 用 `go.mod` 管理依赖，保持最小依赖——只有 gorilla/websocket、gopsutil、go-sql-driver/mysql、golang.org/x/crypto 几个外部包。前端用 `package.json` + `package-lock.json` 锁定版本。Docker 镜像用具体的版本标签（golang:1.25-alpine、nginx:1.27-alpine），不用 latest。

### Q23：有 CI/CD 吗？

**参考回答：** GitHub Actions 自动运行。每次 push 和 PR 触发四个并行 job：Server（Go vet + test -race）、Agent（Go vet + test -race）、Web（TypeScript check + test + build）、Containers（docker compose config 验证 + 镜像构建验证）。

### Q24：你怎么保证代码质量？

**参考回答：** 后端有 50+ 个测试覆盖核心逻辑（API 处理器、WebSocket、数据库 CRUD、并发安全性）。前端有 TypeScript 静态类型检查和 vitest 单元测试。Go 的 race detector 用来检测 WebSocket Hub 的并发竞态问题。每次提交前 CI 自动跑全部检查。

---

## 设计决策类

### Q25：为什么不使用现有的 Web 框架（Gin/Echo/Chi）？

**参考回答：** Go 1.22 的 `net/http` 已经原生支持了模式路由（`mux.HandleFunc("GET /api/devices/{id}", handler)`），不需要第三方路由库。标准库的中间件模式（函数包装 http.Handler）足够灵活。减少外部依赖意味着更少的升级问题、更小的二进制体积、更好的可维护性。

### Q26：为什么用 Zustand 而不是 Redux？

**参考回答：** LabOps 的状态管理需求比较简单——主要是用户认证状态。Zustand 的 API 比 Redux 简洁得多（不需要 action、reducer、dispatch 模板代码），而且内置了 persist 中间件可以直接存到 localStorage。对于一个认证状态 + 几个布尔值来说，Redux 太重了。

### Q27：为什么没有做集群/高可用？

**参考回答：** 项目的定位是个人和小团队使用，通常只有一台服务器。集群和高可用会引入大量复杂度（负载均衡、分布式会话、数据同步），对于当前的用户规模和场景来说是不必要的。如果未来需要，可以将会话存储从本地数据库迁移到 Redis，Server 做无状态水平扩展。

---

## 反思改进类

### Q28：项目中遇到的最大挑战是什么？

**参考回答：** 最大的挑战是 WebSocket 连接管理和并发安全。多台 Agent 同时连接时，需要对 AgentClient 的 map 做并发安全的读写。我用了 sync.RWMutex 保护读多写少的场景，所有 WebSocket 写操作通过 channel 串行化避免竞态，并用 Go 的 race detector 验证没有 data race。

### Q29：如果重新做这个项目，你会怎么改进？

**参考回答：** 三个方向。一是把 Agent 通信层抽象为接口，方便未来支持 gRPC 或 MQTT 等其他协议。二是增加前端 E2E 测试覆盖关键用户流程。三是数据库层目前是手写 SQL，可以考虑引入 sqlc 做类型安全的查询生成。

### Q30：项目有哪些不足？

**参考回答：** 一是前端没有 E2E 测试，目前只有单元测试和类型检查。二是 Agent 目前只支持 Go 实现，对于不想安装 Go 运行时的场景（如嵌入式设备）不够方便。三是监控方面缺少历史数据存储和图表展示，目前只有实时数据。四是缺少多语言支持（目前界面只有中文）。

### Q31：你怎么处理设备离线的情况？

**参考回答：** 两层处理。服务端维护循环每 10 秒检查一次——如果设备最后心跳时间超过 35 秒（可配置），标记为离线。Agent 端使用指数退避自动重连。重连成功后，服务端检查是否有待处理的任务并自动下发。这样即使设备短暂断网，任务也不会丢失。

### Q32：为什么 LLM 提示词用繁体中文？

**参考回答：** 这是设计时的选择，繁体中文在某些 AI 模型中的理解和生成质量上可能略有优势。当前代码库中 UI 界面和规则引擎使用的是简体中文，LLM 提示词使用的是繁体中文。这一点在后续版本中可以考虑统一。

---

## 快速问答

### Q33：项目的默认数据库是什么？
MySQL。需要设置 `LABOPS_DB_DRIVER=sqlite` 才会切换到 SQLite。

### Q34：默认有几个内置命令模板？
5 个，都是 Linux 命令：hostname、uptime、df -h、free -h、ps aux。

### Q35：Agent 多久发一次心跳？
每 10 秒。

### Q36：心跳超时多久标记离线？
35 秒。

### Q37：命令最长执行多久？
300 秒（5 分钟），超时自动终止。

### Q38：输出最多显示多少？
256KB（stdout 和 stderr 各自最多 256KB）。

### Q39：接入码最长有效期？
1 小时。

### Q40：数据库有多少张表？
12 张。
