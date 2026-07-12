# Nginx 问题排查

## Nginx 未启动

**现象：** `curl http://localhost` 返回 `Connection refused`

**检查：**
```bash
sudo systemctl status nginx
```

**修复：** `sudo systemctl start nginx && sudo systemctl enable nginx`

---

## 80 端口未监听

**现象：** 浏览器访问 `http://你的IP` 超时

**检查：**
```bash
sudo ss -lntp | grep :80
```

**正常结果：** 看到 nginx 在 `0.0.0.0:80` 监听

**如果没有：** 检查 Nginx 配置中是否有 `listen 80;`

---

## 配置语法错误

**现象：** `nginx -t` 报错

**常见错误和修复：**
- `unknown directive` → 拼写错误
- `unexpected "}"` → 缺少分号或大括号不匹配
- `host not found in upstream` → proxy_pass 中的域名无法解析

**修复流程：**
1. `sudo nginx -t` 定位错误行
2. 编辑配置文件修正
3. 再次 `sudo nginx -t`
4. 确认语法正确后 `sudo systemctl reload nginx`

---

## 502 Bad Gateway

**现象：** 浏览器显示 502

**含义：** Nginx 能运行，但无法连接到后端（Go Server）

**检查：**
```bash
curl http://localhost:8080/api/health
```

**如果 localhost:8080 也无响应：** Server 容器未运行或崩溃。`docker compose logs server`

---

## 云防火墙未开放

**现象：** 本地 `curl http://localhost` 正常，但外部无法访问

**检查：** 在服务器外执行 `curl http://你的公网IP`

**修复：** 在云厂商安全组中添加入方向规则：TCP 80 端口，授权 0.0.0.0/0
