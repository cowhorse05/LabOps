# Nginx 反向代理与 HTTPS 配置

> **目标读者:** 需要理解和配置网络层的用户
> **前置阅读:** [服务器部署教程](server-deployment.md)

---

## 什么是反向代理

### 直观理解

```
没有反向代理：
  用户浏览器 ──直接访问──► Go Server (:8080)
  问题：没有 HTTPS、需要记住端口号、Server 直接暴露

有了反向代理：
  用户浏览器 ──HTTPS──► Nginx (:443) ──HTTP──► Go Server (:8080)
  好处：HTTPS 加密、统一 443 端口、Server 不直接暴露
```

### 反向代理做什么

1. **接收外部请求** — 监听 80（HTTP）和 443（HTTPS）端口
2. **HTTPS 加解密** — 处理 TLS 握手（对用户加密，对后端解密）
3. **请求转发** — 根据路径决定把请求发往哪里
4. **静态文件** — 直接返回 HTML/JS/CSS，不需经过 Go 后端

---

## 完整 Nginx 配置解析

以下是项目的 Nginx 配置模板（`web/nginx/default.conf.template`），逐段解释：

### HTTP → HTTPS 重定向

```nginx
server {
    listen 80;
    server_name ${SERVER_HOST};    # 你的域名，启动时由 envsubst 替换

    # ACME 验证——certbot 需要这个路径来验证域名控制权
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    # 其他所有请求 → 重定向到 HTTPS
    location / {
        return 301 https://$host$request_uri;
    }
}
```

**关键点：**
- 80 端口只做两件事：ACME 验证 + 重定向
- `301` 是永久重定向，浏览器会记住并自动跳转
- `$host` 和 `$request_uri` 是 Nginx 内置变量，保留原始域名和路径

### HTTPS 主服务

```nginx
server {
    listen 443 ssl http2;              # 开启 SSL 和 HTTP/2
    server_name ${SERVER_HOST};

    # SSL 证书路径
    ssl_certificate     ${TLS_CERT_PATH};     # 证书文件
    ssl_certificate_key ${TLS_KEY_PATH};      # 私钥文件

    # 安全响应头
    add_header Strict-Transport-Security "max-age=31536000" always;  # HSTS
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;
```

**安全头说明：**

| 头部 | 作用 |
|------|------|
| `Strict-Transport-Security` (HSTS) | 告知浏览器「只通过 HTTPS 访问我」，有效期 1 年 |
| `X-Content-Type-Options: nosniff` | 禁止浏览器猜测文件类型（防止 MIME 嗅探攻击） |
| `X-Frame-Options: DENY` | 禁止在 iframe 中加载（防止点击劫持） |

### API 代理（含 WebSocket 升级）

```nginx
    location /api/ {
        proxy_pass http://server:8080;         # 转发到 Go Server
        proxy_http_version 1.1;

        # WebSocket 支持——这两行是关键
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # 传递真实客户端信息
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
```

**WebSocket 为什么需要特殊配置：**

HTTP 和 WebSocket 的握手过程不同：

```
HTTP 请求：
  GET /api/health HTTP/1.1
  → Nginx 正常代理

WebSocket 请求：
  GET /api/agent/ws HTTP/1.1
  Upgrade: websocket              ← 这行表示要升级协议
  Connection: Upgrade             ← 这行表示要保持连接
  → Nginx 需要把这两个头也转发给后端
```

如果不配置 `Upgrade` 和 `Connection` 头，WebSocket 连接会失败，Agent 无法连接到服务端。

### 前端静态文件 + SPA 回退

```nginx
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;
    }
```

**SPA 回退解释：**

React 是一个 SPA（单页应用）。所有路由（如 `/devices/abc123`）实际上都是前端的 JavaScript 处理的。当用户直接访问 `/devices/abc123` 时：

1. Nginx 先尝试匹配实际文件 → 没有这个文件
2. Nginx 尝试匹配目录 → 也不是目录
3. Nginx 回退到 `/index.html` → React Router 接管，显示正确页面

`try_files $uri $uri/ /index.html` 就是实现这个逻辑的。

### 请求体大小限制

```nginx
    client_max_body_size 2m;
```

限制请求体最大 2MB，防止恶意大请求占用内存。

---

## HTTPS 和证书

### 为什么需要证书

HTTPS 需要 SSL/TLS 证书来做两件事：

1. **加密：** 浏览器和服务器之间传输的数据是加密的
2. **身份验证：** 通过证书链证明「这个网站确实是 cowhorse.xyz」，不是中间人伪造的

### Let's Encrypt 证书申请（Certbot）

```bash
sudo certbot --nginx -d cowhorse.xyz -d www.cowhorse.xyz
```

**Certbot 做了什么：**

1. 在 80 端口启动一个临时验证服务
2. Let's Encrypt 服务器访问 `http://cowhorse.xyz/.well-known/acme-challenge/xxx`
3. 验证通过 → 颁发证书
4. Certbot 自动修改 Nginx 配置，添加 ssl_certificate 等指令
5. 证书保存在 `/etc/letsencrypt/live/cowhorse.xyz/`

### 证书文件

| 文件 | 路径 | 用途 |
|------|------|------|
| 完整证书链 | `/etc/letsencrypt/live/cowhorse.xyz/fullchain.pem` | 公开 |
| 私钥 | `/etc/letsencrypt/live/cowhorse.xyz/privkey.pem` | **绝对不能泄露** |

### 自动续期

Let's Encrypt 证书 90 天过期。Certbot 自动安装了续期定时器：

```bash
# 测试续期是否正常（不会实际续期）
sudo certbot renew --dry-run

# 查看自动续期定时器
sudo systemctl status certbot.timer
```

### 手动续期

```bash
sudo certbot renew
```

### 续期后重载 Nginx

证书续期后需要让 Nginx 加载新证书。如果你配置了 deploy hook：

```bash
# Certbot 自动调用这个钩子
sudo nginx -s reload
```

如果使用了 Docker 中运行的 Nginx（如项目的 compose.yaml 部署），则需要在宿主机上创建续期钩子。

---

## 常见问题排查

### 80 端口被占用

**现象：** Nginx 无法启动，提示 `bind() to 0.0.0.0:80 failed`

**检查：** `sudo ss -lntp | grep :80`

**修复：** 停止占用 80 端口的程序

### 证书申请失败

**现象：** certbot 报 `Connection refused` 或 `timeout`

**原因：** 80 端口未对外开放或 Nginx 未运行

**检查：**
- `curl http://你的域名/.well-known/acme-challenge/test`
- 在服务器外（你的本地电脑）执行，确认 80 端口公网可达

### HTTPS 可以访问但显示不安全

**原因：** 证书过期或域名不匹配

**检查：** 浏览器点击地址栏的锁图标，查看证书详情

**修复：** `sudo certbot renew` 或重新申请证书
