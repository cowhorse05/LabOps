# SSH 连接问题排查

## SSH 无法连接

**现象：** `ssh: connect to host 47.96.xxx.xxx port 22: Connection refused` 或 `Connection timed out`

**可能原因：**
1. 安全组/防火墙未开放 22 端口
2. SSH 服务未运行
3. 使用了错误的端口

**检查命令：**
```bash
# 在服务器上检查 SSH 是否在监听
sudo ss -lntp | grep :22
```

**正常结果：** 看到 sshd 在 22 端口监听

**修复：** `sudo systemctl start sshd`，检查云安全组是否开放 22 端口

---

## SSH 用户名错误

**现象：** `admin@47.96.xxx.xxx: Permission denied (publickey).`

**可能原因：** 使用了错误的用户名

**检查：** 查看云厂商控制台确认正确的登录用户名。阿里云 ECS 通常是 `root`，轻量服务器使用创建时设置的用户名

---

## 私钥不匹配

**现象：** `Permission denied (publickey).` 但用户名正确

**检查：**
```bash
ssh -v -i 你的私钥路径 用户名@IP
# 查看 debug 输出中的 "Offering public key" 是否被服务器接受
```

**修复：** 确认本地私钥和服务器上的公钥是一对。在云厂商控制台重置密钥对

---

## Permission denied（密码方式）

**可能原因：** 密码错误、密码登录被禁用

**检查：** 在云厂商控制台重置密码。确认 SSH 配置中 `PasswordAuthentication yes`

---

## Windows 私钥权限问题

**现象：** `UNPROTECTED PRIVATE KEY FILE!`

**修复（PowerShell）：**
```powershell
icacls 私钥文件 /inheritance:r /grant "${env:USERNAME}:R"
```
