# Docker 问题排查

## 容器启动失败

**现象：** `docker compose up -d` 后某个服务状态不是 Up

**检查：**
```bash
docker compose ps          # 查看哪些容器没起来
docker compose logs 服务名  # 查看具体错误
```

**常见原因：**
- 环境变量未设置或格式错误
- 端口被占用
- MySQL 未就绪（等待健康检查）

---

## 端口被占用

**现象：** `bind: address already in use`

**检查：**
```bash
sudo ss -lntp | grep -E ':80 |:443 |:8080 |:3306 '
```

**修复：** 停止占用端口的程序，或修改端口映射

---

## 环境变量缺失

**现象：** 容器启动后立即退出，日志显示 `variable not set`

**检查：** `.env` 文件中所有必填变量是否已设置（特别是带 `?set` 后缀的）

---

## 数据目录权限错误

**现象：** MySQL 容器无法启动，日志显示 `Permission denied`

**检查：**
```bash
docker volume inspect labops_mysql-data
# 查看 Mountpoint 路径的权限
```

**修复：** 删除并重建 Volume（⚠️ 会丢失数据）或修复权限

---

## Docker 服务未运行

**检查：**
```bash
sudo systemctl status docker
```

**修复：** `sudo systemctl start docker && sudo systemctl enable docker`

## 不能免 sudo 使用 docker

**修复：**
```bash
sudo usermod -aG docker $USER
# 退出重新登录后生效
```
