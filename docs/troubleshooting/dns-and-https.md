# DNS 和 HTTPS 问题排查

## 域名无法解析

**现象：** `nslookup cowhorse.xyz` 返回 `server can't find cowhorse.xyz`

**检查：**
```bash
nslookup cowhorse.xyz 8.8.8.8   # 用 Google DNS 查询
```

**可能原因：**
- A 记录还没生效（新增记录可能需要几分钟到几小时）
- 域名过期
- DNS 服务器配置错误

---

## 公网 IP 可以访问但域名不行

**现象：** `http://你的IP` 正常，但 `http://你的域名` 超时

**原因：** DNS 还没生效或 A 记录指向错误的 IP

**修复：** 确认 A 记录的值是你的服务器 IP。等 DNS 生效。

---

## ICP 备案拦截

**现象：** 中国大陆服务器上的域名在浏览器中显示备案提示页面

**原因：** 域名解析到中国大陆服务器但未完成 ICP 备案

**说明：** 这是中国大陆的法规要求，和 SSL 证书、DNS 配置无关。

**解决：**
- 完成 ICP 备案（1-3 周）
- 或使用海外服务器（通常不需要备案）
- 备案期间可通过 IP 直接访问

---

## HTTPS 证书申请失败

**现象：** `certbot` 报错，常见信息：

- `Connection refused` → 80 端口未开放
- `DNS problem: NXDOMAIN` → 域名 DNS 未生效
- `Invalid response from ...` → Nginx 或防火墙配置问题

**检查清单：**
```bash
# 1. 确认 80 端口公网可达（在你的本地电脑执行）
curl http://你的域名

# 2. 确认 ACME 验证路径可访问
curl http://你的域名/.well-known/acme-challenge/test

# 3. 确认域名 DNS 正确
nslookup 你的域名
```

---

## 证书续期失败

**现象：** 证书过期后网站显示不安全

**检查：**
```bash
sudo certbot renew --dry-run     # 测试续期
sudo systemctl status certbot.timer  # 检查定时器
```

**修复：**
```bash
sudo certbot renew               # 手动续期
sudo systemctl reload nginx      # 重载 Nginx
```

---

## 浏览器显示「不安全」

**原因：**
- 证书过期
- 证书域名和访问的域名不匹配
- 使用了自签名证书

**检查：** 点击浏览器地址栏的锁图标查看证书详情

**修复：** 重新申请证书或等待自动续期
