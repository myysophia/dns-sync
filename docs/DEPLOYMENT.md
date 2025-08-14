# DNS增量同步部署指南

## 编译二进制文件

```bash
# 在项目根目录执行
go build -o dns-sync-incremental main_incremental.go

# 验证编译结果
./dns-sync-incremental
```

## 服务器部署

### 1. 创建目录结构

```bash
# 创建应用目录
sudo mkdir -p /opt/dns-sync/{config,logs}

# 设置权限
sudo chown -R $USER:$USER /opt/dns-sync
```

### 2. 上传文件

```bash
# 上传二进制文件
scp dns-sync-incremental user@server:/opt/dns-sync/

# 上传配置文件
scp config/config.yaml user@server:/opt/dns-sync/config/

# 设置执行权限
chmod +x /opt/dns-sync/dns-sync-incremental
```

### 3. 配置定时任务

```bash
# 编辑crontab
crontab -e

# 添加以下行（每天凌晨2点执行）
0 2 * * * cd /opt/dns-sync && ./dns-sync-incremental >> logs/dns-sync-$(date +\%Y\%m\%d).log 2>&1

# 或者使用更详细的日志
0 2 * * * cd /opt/dns-sync && echo "=== $(date) DNS同步开始 ===" >> logs/dns-sync.log && ./dns-sync-incremental >> logs/dns-sync.log 2>&1 && echo "=== $(date) DNS同步结束 ===" >> logs/dns-sync.log
```

### 4. 测试运行

```bash
# 手动测试
cd /opt/dns-sync
./dns-sync-incremental

# 检查日志
tail -f logs/dns-sync.log
```

## 日志管理

### 自动清理旧日志

```bash
# 添加日志清理任务（每周清理30天前的日志）
0 3 * * 0 find /opt/dns-sync/logs -name "*.log" -mtime +30 -delete
```

### 日志轮转配置

创建 `/etc/logrotate.d/dns-sync`:

```
/opt/dns-sync/logs/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 $USER $USER
}
```

## 监控和告警

### 1. 检查同步状态脚本

创建 `/opt/dns-sync/check_sync.sh`:

```bash
#!/bin/bash
LOG_FILE="/opt/dns-sync/logs/dns-sync.log"
TODAY=$(date +%Y-%m-%d)

if grep -q "$TODAY.*DNS INCREMENTAL SYNC SUMMARY" "$LOG_FILE" && \
   grep -q "$TODAY.*Failed: 0" "$LOG_FILE"; then
    echo "DNS同步正常"
    exit 0
else
    echo "DNS同步异常，请检查日志"
    exit 1
fi
```

### 2. 添加健康检查

```bash
# 每天上午9点检查昨晚的同步状态
0 9 * * * /opt/dns-sync/check_sync.sh || echo "DNS同步失败" | mail -s "DNS同步告警" admin@company.com
```

## 配置文件管理

### 生产环境配置示例

```yaml
# /opt/dns-sync/config/config.yaml
aliyun:
  access_key_id: "${ALIYUN_ACCESS_KEY_ID}"
  access_key_secret: "${ALIYUN_ACCESS_KEY_SECRET}"
  region: "cn-hangzhou"

mysql:
  host: "prod-mysql.internal"
  port: 3306
  username: "${DB_USERNAME}"
  password: "${DB_PASSWORD}"
  database: "jeecg-boot"

domains:
  - project_id: "1955529112922935297"
    domain_id: "1955529700108718082"
    domain: "pingjl.com"
  # ... 其他域名配置
```

### 环境变量设置

```bash
# 在 ~/.bashrc 或 /etc/environment 中设置
export ALIYUN_ACCESS_KEY_ID="your_access_key"
export ALIYUN_ACCESS_KEY_SECRET="your_secret_key"
export DB_USERNAME="dns_sync_user"
export DB_PASSWORD="your_db_password"
```

## 故障排除

### 常见问题

1. **权限问题**
   ```bash
   chmod +x /opt/dns-sync/dns-sync-incremental
   chown $USER:$USER /opt/dns-sync -R
   ```

2. **网络连接问题**
   ```bash
   # 测试阿里云连接
   curl -I https://alidns.aliyuncs.com
   
   # 测试数据库连接
   mysql -h your_host -u your_user -p
   ```

3. **配置文件问题**
   ```bash
   # 验证YAML格式
   python -c "import yaml; yaml.safe_load(open('/opt/dns-sync/config/config.yaml'))"
   ```

### 日志分析

```bash
# 查看最近的同步结果
grep "DNS INCREMENTAL SYNC SUMMARY" /opt/dns-sync/logs/dns-sync.log | tail -5

# 查看错误信息
grep -i "error\|failed" /opt/dns-sync/logs/dns-sync.log | tail -10

# 统计同步记录数
grep "sync completed" /opt/dns-sync/logs/dns-sync.log | tail -10
```

## 性能优化

### 1. 数据库连接池优化

程序已经配置了合理的连接池参数：
- MaxOpenConns: 25
- MaxIdleConns: 10
- ConnMaxLifetime: 5分钟

### 2. 批量操作优化

程序使用事务和批量操作，确保高效的数据库写入。

### 3. 内存使用优化

程序运行完成后自动释放资源，适合定时任务场景。

## 安全建议

1. **最小权限原则**：数据库用户只授予必要的权限
2. **配置文件保护**：设置适当的文件权限（600）
3. **日志敏感信息**：避免在日志中记录敏感信息
4. **网络安全**：使用VPN或内网连接数据库

## 备份和恢复

### 配置备份

```bash
# 定期备份配置文件
0 4 * * * cp /opt/dns-sync/config/config.yaml /opt/dns-sync/config/config.yaml.$(date +\%Y\%m\%d)
```

### 数据库备份

建议在同步前进行数据库备份，以防数据异常。
