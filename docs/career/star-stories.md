# STAR 案例集

> **目标读者:** 面试准备者
> **说明:** 每个案例遵循 STAR 格式（Situation → Task → Action → Result），基于项目真实经历。

---

## 案例 1：首次完成服务器部署

**Situation（情境）：**
项目代码开发完成后，需要将其部署到真实的云服务器上，让其他人可以通过公网访问。之前只有本地开发经验，对 Linux 服务器运维、Docker 生产部署、Nginx 配置都不熟悉。

**Task（任务）：**
在阿里云 Linux 服务器上部署 LabOps，实现 HTTPS 访问、数据库持久化、Agent 远程连接。

**Action（行动）：**
1. 学习 Linux 基础命令（包管理器、systemd、文件权限）
2. 安装 Docker 和 Docker Compose，编写 compose.yaml 配置三服务协作
3. 配置 Nginx 反向代理（HTTP→HTTPS 重定向、API 代理、WebSocket 升级、SPA 回退）
4. 申请 Let's Encrypt SSL 证书，配置自动续期
5. 解决部署过程中遇到的问题（端口冲突、权限错误、防火墙配置）

**Result（结果）：**
- 项目成功部署在公网服务器上，通过 `https://cowhorse.xyz` 访问
- 掌握了从代码到生产的完整部署技能
- 将整个过程整理成文档，可以指导他人完成同样的部署

**体现的能力：** Linux 运维、Docker、Nginx、HTTPS、问题排查、文档编写

---

## 案例 2：解决 Nginx WebSocket 代理问题

**Situation：**
Agent 使用 WebSocket 连接服务端。本地开发时直连 Server 端口正常，但部署到服务器通过 Nginx 代理后，Agent 无法建立 WebSocket 连接。

**Task：**
排查并修复 Nginx 代理 WebSocket 的问题。

**Action：**
1. 查看 Agent 日志 → 连接被 Nginx 返回 400 Bad Request
2. 研究 WebSocket 协议 → 发现 WebSocket 握手需要 `Upgrade: websocket` 和 `Connection: Upgrade` 头
3. 检查 Nginx 配置 → 发现只配了 `proxy_pass`，没有转发升级头
4. 添加 `proxy_set_header Upgrade $http_upgrade` 和 `proxy_set_header Connection "upgrade"`
5. 测试确认 WebSocket 升级成功

**Result：**
- Agent 通过 Nginx 的 WSS 连接正常工作
- 理解了 WebSocket 协议的升级机制
- 在文档中详细解释了 WebSocket 代理的配置要点

**体现的能力：** 协议理解、问题排查、Nginx 配置

---

## 案例 3：解决 DNS 和备案问题

**Situation：**
使用中国内地（大陆）阿里云服务器时，域名无法正常访问。浏览器显示的是备案拦截页面。

**Task：**
让域名能正常解析到服务器并访问项目。

**Action：**
1. 排查：确认 DNS A 记录正确、服务器 80 端口可访问
2. 发现：中国大陆服务器要求域名完成 ICP 备案才能通过 80/443 端口访问
3. 临时方案：备案期间使用 IP 地址直接访问
4. 了解备案流程和要求
5. 理解海外服务器通常不需要备案的区别

**Result：**
- 明确了备案和 DNS 是两个独立的问题
- 了解了中国大陆服务器的合规要求
- 在文档中说明了备案问题和解决方案

**体现的能力：** 问题定位、法规了解、合规意识

---

## 案例 4：配置 SSH 密钥登录

**Situation：**
每次部署和运维都需要输入密码登录服务器，效率低且不安全。

**Task：**
配置 SSH 密钥登录，实现免密安全连接。

**Action：**
1. 在本地生成 SSH 密钥对：`ssh-keygen -t ed25519`
2. 将公钥上传到服务器：`ssh-copy-id 用户名@服务器IP`
3. 测试密钥登录：`ssh -i ~/.ssh/id_ed25519 用户名@服务器IP`
4. 配置 `~/.ssh/config` 简化连接命令
5. 可选：禁用密码登录提高安全性

**Result：**
- 实现了安全的免密 SSH 登录
- 理解了公钥/私钥的认证原理
- 在部署文档中说明了密钥登录的详细步骤

**体现的能力：** SSH 安全配置、公钥加密理解

---

## 案例 5：配置 Agent systemd 服务

**Situation：**
Agent 在设备上手动运行时一切正常，但终端关闭后 Agent 就停止了。需要一个方法让 Agent 在后台持续运行，并且开机自动启动。

**Task：**
使用 systemd 将 Agent 配置为系统服务，实现后台运行、自动重启、开机自启。

**Action：**
1. 学习 systemd 服务文件的编写
2. 创建 `labops-agent.service`，配置 `Restart=on-failure` 和 `RestartSec=5s`
3. 研究 systemd 的安全限制选项
4. 添加 10 项安全加固配置（NoNewPrivileges、ProtectSystem=strict 等）
5. 测试服务在各种异常情况下的行为（进程崩溃、系统重启）

**Result：**
- Agent 以系统服务方式稳定运行
- 进程崩溃后 5 秒自动重启
- 系统重启后自动启动 Agent
- 安全加固限制了 Agent 的权限范围

**体现的能力：** systemd 管理、Linux 安全配置、服务设计

---

## 案例 6：完成数据备份恢复

**Situation：**
项目已经上线并有真实数据。需要建立备份机制防止数据丢失，并验证备份可以恢复。

**Task：**
实现自动化数据库备份和恢复流程。

**Action：**
1. 编写备份脚本：使用 mysqldump 导出 MySQL 数据库，gzip 压缩
2. 实现保留策略：7 天日备份 + 4 周周备份
3. 配置 systemd timer 实现每天凌晨 3:15 自动备份
4. 编写恢复脚本：添加安全确认机制（必须设置环境变量才能执行）
5. 执行恢复演练：在测试数据库中验证备份文件可恢复

**Result：**
- 建立了自动化的每日备份机制
- 备份和恢复脚本有安全防护防止误操作
- 通过恢复演练验证了备份的可用性

**体现的能力：** 数据库管理、Shell 脚本、运维自动化

---

## 案例 7：排查 Docker 端口冲突

**Situation：**
新部署时，`docker compose up -d` 报错 `bind: address already in use`。80 端口被占用。

**Task：**
找出占用 80 端口的程序并解决冲突。

**Action：**
1. `sudo ss -lntp | grep :80` → 发现已有的 Nginx 在占用 80 端口
2. 分析：主机上的 Nginx 和 Docker 中的 Nginx 冲突
3. 决策：使用主机的 Nginx 做反向代理（修改配置指向 Docker 内的 Server）
4. 修改 Nginx 配置 → `proxy_pass http://localhost:8080`
5. 移除 Docker Compose 中 Web 服务的 80 端口映射

**Result：**
- 解决了端口冲突，服务正常运行
- 理解了宿主机服务与 Docker 容器之间的网络关系
- 在文档中记录了排查步骤

**体现的能力：** 问题排查、网络理解、运维决策

---

## 案例 8：跨系统兼容（apt vs dnf）

**Situation：**
部署文档最初只写了 Ubuntu（apt）的命令。当需要在 Alibaba Cloud Linux（dnf）服务器上部署时，命令全部不适用。

**Task：**
更新部署文档，让使用不同 Linux 发行版的用户都能按文档操作。

**Action：**
1. 研究不同 Linux 发行版的包管理器差异（apt vs dnf vs pacman）
2. 在文档中每条安装命令提供两个版本
3. 添加「如何判断你的系统」章节
4. 添加「查看操作系统信息」的教学步骤
5. 使用 `cat /etc/os-release` 帮助用户判断应该用哪个命令

**Result：**
- 部署文档同时支持 Ubuntu/Debian（apt）和 Alibaba Cloud Linux/RHEL 系（dnf）
- 新手用户可以自行判断应该使用哪条命令
- 文档更具通用性和实用性

**体现的能力：** 跨平台思维、文档编写、用户导向
