# Crush JWT 认证系统 - 部署指南

## 📦 生产环境部署

### 前置准备

#### 1. 系统要求
- Linux 服务器（Ubuntu 20.04+ 或 CentOS 8+）
- Go 1.25.0+
- Node.js 18+
- Nginx（用于反向代理）
- SSL 证书（推荐使用 Let's Encrypt）

#### 2. 安全清单
- [ ] 更改默认的 JWT Secret
- [ ] 使用 bcrypt 替换 SHA-256 密码哈希
- [ ] 将用户数据迁移到数据库
- [ ] 配置 HTTPS/WSS
- [ ] 限制 CORS 到特定域名
- [ ] 设置防火墙规则
- [ ] 配置日志轮转
- [ ] 设置监控和告警

## 🔧 后端部署

### 1. 编译后端

```bash
cd crush-main

# 编译生产版本
CGO_ENABLED=1 go build -ldflags="-s -w" -o crush main.go

# 验证编译
./crush --version
```

### 2. 配置环境变量

创建 `/etc/crush/config.env`:

```bash
# JWT 配置 - 必须修改！
export JWT_SECRET="your-very-long-and-secure-secret-key-at-least-32-characters"

# 服务器配置
export HTTP_PORT=8081
export WEBSOCKET_PORT=8080

# CORS 配置 - 限制到你的域名
export CORS_ALLOWED_ORIGINS="https://yourdomain.com"

# 日志配置
export LOG_LEVEL=info
export LOG_FILE=/var/log/crush/crush.log

# 数据库配置（如果使用）
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=crush
export DB_USER=crush_user
export DB_PASSWORD=your_secure_db_password
```

### 3. 创建 Systemd 服务

创建 `/etc/systemd/system/crush.service`:

```ini
[Unit]
Description=Crush AI Assistant Server
After=network.target

[Service]
Type=simple
User=crush
Group=crush
WorkingDirectory=/opt/crush
EnvironmentFile=/etc/crush/config.env
ExecStart=/opt/crush/crush
Restart=always
RestartSec=10

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/crush /opt/crush/data

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

### 4. 启动服务

```bash
# 创建用户和目录
sudo useradd -r -s /bin/false crush
sudo mkdir -p /opt/crush /var/log/crush /opt/crush/data
sudo chown -R crush:crush /opt/crush /var/log/crush

# 复制编译好的二进制文件
sudo cp crush /opt/crush/
sudo chmod +x /opt/crush/crush

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable crush
sudo systemctl start crush

# 检查状态
sudo systemctl status crush
sudo journalctl -u crush -f
```

## 🌐 前端部署

### 1. 构建前端

```bash
cd crush-fe

# 安装依赖
pnpm install

# 构建生产版本
pnpm build

# 输出在 dist/ 目录
ls -la dist/
```

### 2. 配置 Nginx

创建 `/etc/nginx/sites-available/crush`:

```nginx
# HTTP -> HTTPS 重定向
server {
    listen 80;
    listen [::]:80;
    server_name yourdomain.com;
    
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    
    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS 主服务器
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name yourdomain.com;
    
    # SSL 证书
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    
    # SSL 配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    
    # 安全头
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # 前端静态文件
    root /var/www/crush;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    # API 代理
    location /api/ {
        proxy_pass http://localhost:8081/api/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # WebSocket 代理
    location /ws {
        proxy_pass http://localhost:8080/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 超时设置
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
    }
    
    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

### 3. 部署前端文件

```bash
# 创建目录
sudo mkdir -p /var/www/crush

# 复制构建文件
sudo cp -r dist/* /var/www/crush/

# 设置权限
sudo chown -R www-data:www-data /var/www/crush
sudo chmod -R 755 /var/www/crush

# 启用站点
sudo ln -s /etc/nginx/sites-available/crush /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## 🔒 SSL 证书配置

### 使用 Let's Encrypt

```bash
# 安装 Certbot
sudo apt-get update
sudo apt-get install certbot python3-certbot-nginx

# 获取证书
sudo certbot --nginx -d yourdomain.com

# 自动续期
sudo certbot renew --dry-run

# 添加 cron 任务自动续期
echo "0 3 * * * certbot renew --quiet" | sudo crontab -
```

## 🗄️ 数据库配置（推荐）

### PostgreSQL 设置

```bash
# 安装 PostgreSQL
sudo apt-get install postgresql postgresql-contrib

# 创建数据库和用户
sudo -u postgres psql << EOF
CREATE DATABASE crush;
CREATE USER crush_user WITH ENCRYPTED PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE crush TO crush_user;
\q
EOF

# 创建用户表
sudo -u postgres psql -d crush << EOF
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
EOF
```

## 🔥 防火墙配置

```bash
# UFW 配置
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 80/tcp      # HTTP
sudo ufw allow 443/tcp     # HTTPS
sudo ufw enable

# 内部端口不对外开放
# 8080 和 8081 只通过 Nginx 代理访问
```

## 📊 监控和日志

### 1. 日志轮转

创建 `/etc/logrotate.d/crush`:

```
/var/log/crush/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0644 crush crush
    sharedscripts
    postrotate
        systemctl reload crush > /dev/null 2>&1 || true
    endscript
}
```

### 2. 监控脚本

创建 `/opt/crush/monitor.sh`:

```bash
#!/bin/bash

# 检查服务状态
if ! systemctl is-active --quiet crush; then
    echo "Crush service is down! Restarting..."
    systemctl restart crush
    # 发送告警邮件或通知
fi

# 检查端口
if ! nc -z localhost 8081; then
    echo "HTTP port 8081 is not responding!"
fi

if ! nc -z localhost 8080; then
    echo "WebSocket port 8080 is not responding!"
fi
```

添加到 crontab:
```bash
*/5 * * * * /opt/crush/monitor.sh >> /var/log/crush/monitor.log 2>&1
```

## 🚀 性能优化

### 1. Go 服务器优化

```go
// 在 main.go 中添加
import "runtime"

func init() {
    // 设置 Go 运行时参数
    runtime.GOMAXPROCS(runtime.NumCPU())
}
```

### 2. Nginx 优化

在 `/etc/nginx/nginx.conf` 中:

```nginx
worker_processes auto;
worker_rlimit_nofile 65535;

events {
    worker_connections 4096;
    use epoll;
    multi_accept on;
}

http {
    # 启用 gzip 压缩
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript 
               application/json application/javascript application/xml+rss;
    
    # 连接优化
    keepalive_timeout 65;
    keepalive_requests 100;
    
    # 缓冲区优化
    client_body_buffer_size 128k;
    client_max_body_size 10m;
    client_header_buffer_size 1k;
    large_client_header_buffers 4 4k;
    output_buffers 1 32k;
    postpone_output 1460;
}
```

## 🔍 故障排查

### 检查服务状态
```bash
# 后端服务
sudo systemctl status crush
sudo journalctl -u crush -n 100 --no-pager

# Nginx
sudo systemctl status nginx
sudo nginx -t
sudo tail -f /var/log/nginx/error.log

# 检查端口
sudo netstat -tlnp | grep -E '8080|8081|443'
```

### 常见问题

#### 1. WebSocket 连接失败
- 检查 Nginx WebSocket 代理配置
- 确认防火墙允许 443 端口
- 验证 SSL 证书有效

#### 2. 后端服务无法启动
- 检查端口是否被占用
- 验证环境变量配置
- 查看日志文件

#### 3. 前端无法加载
- 检查 Nginx 配置
- 验证文件权限
- 清除浏览器缓存

## 📈 扩展性考虑

### 负载均衡

如果需要处理大量并发连接，可以使用多个后端实例：

```nginx
upstream crush_backend {
    least_conn;
    server localhost:8081;
    server localhost:8082;
    server localhost:8083;
}

upstream crush_websocket {
    ip_hash;  # WebSocket 需要会话粘性
    server localhost:8080;
    server localhost:8090;
    server localhost:8100;
}
```

### Redis 会话存储

使用 Redis 存储 JWT token 和会话信息：

```go
import "github.com/go-redis/redis/v8"

var rdb = redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// 存储 token
rdb.Set(ctx, "token:"+userID, token, 24*time.Hour)

// 验证 token
val, err := rdb.Get(ctx, "token:"+userID).Result()
```

## ✅ 部署检查清单

部署前确认：

- [ ] 修改了默认的 JWT Secret
- [ ] 配置了 HTTPS/WSS
- [ ] 限制了 CORS 到特定域名
- [ ] 设置了防火墙规则
- [ ] 配置了日志轮转
- [ ] 设置了监控脚本
- [ ] 测试了所有功能
- [ ] 备份了配置文件
- [ ] 准备了回滚方案
- [ ] 文档已更新

部署后验证：

- [ ] 可以通过 HTTPS 访问前端
- [ ] 登录功能正常
- [ ] WebSocket 连接正常
- [ ] 日志正常记录
- [ ] 监控正常运行
- [ ] SSL 证书有效
- [ ] 性能符合预期

## 🆘 紧急回滚

如果部署出现问题：

```bash
# 停止新版本
sudo systemctl stop crush

# 恢复旧版本
sudo cp /opt/crush/crush.backup /opt/crush/crush

# 重启服务
sudo systemctl start crush

# 检查状态
sudo systemctl status crush
```

## 📞 支持

如有问题，请查看：
- [快速启动指南](./QUICK_START_GUIDE.md)
- [实现文档](./JWT_AUTH_IMPLEMENTATION.md)
- 项目 GitHub Issues

---

**注意**: 本指南提供了基本的生产部署步骤。根据你的具体需求和环境，可能需要进行调整。

