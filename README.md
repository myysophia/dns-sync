# DNS Sync Tool

将阿里云DNS解析记录同步到MySQL数据库的工具。

## 功能特性

- 使用阿里云SDK v2.0获取域名DNS记录
- 批量同步多个域名的DNS记录到MySQL数据库
- 支持配置文件管理阿里云凭证和数据库连接
- 完整的错误处理和日志记录
- 事务支持，确保数据一致性
- 自动清理旧记录，避免重复数据

## 项目结构

```
dns-sync/
├── config/
│   └── config.yaml        # 配置文件
├── internal/
│   ├── config/            # 配置管理
│   │   └── config.go
│   ├── aliyun/           # 阿里云DNS SDK
│   │   └── dns_client.go
│   ├── database/         # MySQL数据库操作
│   │   └── mysql.go
│   └── models/           # 数据模型
│       └── models.go
├── go.mod
├── go.sum
├── main.go
└── README.md
```

## 环境要求

- Go 1.23.3 或更高版本
- MySQL 数据库
- 阿里云DNS服务访问权限

## 安装和配置

### 1. 克隆项目

```bash
git clone <repository-url>
cd dns-sync
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 配置文件

复制并编辑配置文件 `config/config.yaml`：

```yaml
aliyun:
  access_key_id: "your_access_key_id"        # 阿里云AccessKey ID
  access_key_secret: "your_access_key_secret" # 阿里云AccessKey Secret
  region: "cn-hangzhou"                       # 区域

mysql:
  host: "localhost"      # MySQL主机地址
  port: 3306            # MySQL端口
  username: "root"      # MySQL用户名
  password: "password"  # MySQL密码
  database: "jeecg-boot" # 数据库名

domains:
  - project_id: "1955529112922935297"
    domain_id: "1955529700108718082"
    domain: "pingjl.com"
  - project_id: "1955529112922935297"
    domain_id: "1955529700129689602"
    domain: "vnnox.com"
  # 添加更多域名映射...
```

### 4. 数据库表结构

确保MySQL数据库中存在 `asset_sub_domain` 表：

```sql
CREATE TABLE IF NOT EXISTS `asset_sub_domain` (
  `id` varchar(50) NOT NULL COMMENT 'ID',
  `sub_domain` varchar(255) DEFAULT NULL COMMENT '子域名',
  `type` varchar(10) DEFAULT NULL COMMENT 'DNS记录类型',
  `create_time` datetime DEFAULT NULL COMMENT '创建时间',
  `update_by` varchar(50) DEFAULT NULL COMMENT '更新人',
  `create_by` varchar(50) DEFAULT NULL COMMENT '创建人',
  `update_time` datetime DEFAULT NULL COMMENT '更新时间',
  `sys_org_code` varchar(50) DEFAULT NULL COMMENT '组织代码',
  `dns_record` varchar(255) DEFAULT NULL COMMENT 'DNS记录',
  `name_server` varchar(255) DEFAULT NULL COMMENT '域名服务器',
  `asset_label` varchar(255) DEFAULT '' COMMENT '资产标签',
  `asset_manager` varchar(50) DEFAULT NULL COMMENT '资产管理员',
  `asset_department` varchar(100) DEFAULT NULL COMMENT '资产部门',
  `level` varchar(20) DEFAULT NULL COMMENT '级别',
  `domain_id` varchar(50) DEFAULT NULL COMMENT '域名ID',
  `source` varchar(50) DEFAULT NULL COMMENT '数据来源',
  `project_id` varchar(50) DEFAULT NULL COMMENT '项目ID',
  PRIMARY KEY (`id`),
  KEY `idx_domain_id` (`domain_id`),
  KEY `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='子域名资产表';
```

## 使用方法

### 运行同步程序

```bash
go run main.go
```

### 编译二进制文件

```bash
# Windows
go build -o dns-sync.exe main.go

# Linux/Mac
go build -o dns-sync main.go
```

## 数据映射说明

| 阿里云DNS字段 | 数据库字段 | 说明 |
|------------|-----------|------|
| RR + DomainName | sub_domain | 子域名（如：www.example.com） |
| Type | type | DNS记录类型（A, CNAME, MX等） |
| - | domain_id | 从配置文件映射获取 |
| - | project_id | 从配置文件映射获取 |
| - | source | 固定值："Aliyun-DNS-Sync" |
| - | create_time | 当前时间 |
| - | update_time | 当前时间 |

## 日志和监控

程序会输出详细的同步日志，包括：
- 配置加载状态
- 连接测试结果
- 每个域名的处理进度
- 同步结果摘要

## 错误处理

- 配置文件验证
- 网络连接失败重试
- 数据库事务回滚
- 详细错误日志记录

## 注意事项

1. **权限要求**：确保阿里云AccessKey有DNS服务的读取权限
2. **数据覆盖**：程序会清除现有的同源记录，避免重复数据
3. **批量操作**：使用事务确保数据一致性
4. **ID生成**：使用时间戳生成ID，可根据需要修改为雪花算法等

## 故障排除

### 常见问题

1. **阿里云认证失败**
   - 检查AccessKey ID和Secret是否正确
   - 确认账号有DNS服务权限

2. **数据库连接失败**
   - 检查MySQL服务是否启动
   - 确认连接参数是否正确
   - 检查防火墙设置

3. **表不存在错误**
   - 确认数据库中存在asset_sub_domain表
   - 检查表结构是否匹配

## 开发说明

项目采用模块化设计，各模块职责清晰：
- `config`: 配置管理
- `aliyun`: 阿里云DNS API封装
- `database`: MySQL数据库操作
- `models`: 数据模型定义

## 许可证

[MIT]
