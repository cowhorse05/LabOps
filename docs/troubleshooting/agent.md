# Agent 问题排查

## Agent 无法连接 Server

**现象：** `journalctl -u labops-agent` 显示连接错误

**检查：**
```bash
# 1. 确认凭据文件存在
sudo ls -la /etc/labops-agent/credentials.json

# 2. 确认环境配置
sudo cat /etc/labops-agent/agent.env

# 3. 确认网络可达
curl https://你的域名/api/health
```

**可能原因：**
- 服务器 URL 配置错误
- 凭据文件丢失或损坏
- 设备无法访问外网 443 端口
- 服务器防火墙拦截

**修复：** 重新执行接入流程

---

## Agent 显示离线

**现象：** Web 界面显示设备离线

**检查：**
```bash
sudo systemctl status labops-agent
```

**可能原因：**
- Agent 服务停止
- 网络中断
- 凭据被吊销

**修复：**
```bash
sudo systemctl restart labops-agent
sudo journalctl -u labops-agent -f
```

---

## Systemd 服务启动失败

**现象：** `systemctl start labops-agent` 失败

**检查：**
```bash
sudo journalctl -u labops-agent -n 50
```

**常见原因：**
- 二进制文件不存在：`ls -la /usr/local/bin/labops-agent`
- 环境变量文件缺失：`ls -la /etc/labops-agent/agent.env`
- 凭据文件缺失：`ls -la /etc/labops-agent/credentials.json`
- 二进制不可执行：`chmod +x /usr/local/bin/labops-agent`

---

## 凭证过期或无效

**现象：** 日志显示认证失败

**修复：**
1. 在 Web 界面吊销该设备的凭据
2. 生成新的接入码
3. 在设备上重新接入：
```bash
sudo /usr/local/bin/labops-agent \
  --enroll-only --enroll-code "新接入码" \
  --server "https://你的域名" --real \
  --credentials /etc/labops-agent/credentials.json
sudo systemctl restart labops-agent
```

---

## Agent 日志中出现大量重连

**现象：** `journalctl -u labops-agent` 反复显示 `connecting` 和 `disconnected`

**原因：** 网络不稳定或服务器临时不可用

**说明：** Agent 使用指数退避重连（1s → 2s → 4s → ... → 60s 最大），这是**正常行为**。如果持续超过几分钟，检查网络。
