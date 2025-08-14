package database

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"dns-sync/internal/config"
	"dns-sync/internal/models"
)

// MySQLClient MySQL客户端
type MySQLClient struct {
	db *sql.DB
}

// NewMySQLClient 创建MySQL客户端
func NewMySQLClient(cfg *config.MySQLConfig) (*MySQLClient, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &MySQLClient{
		db: db,
	}, nil
}

// Close 关闭数据库连接
func (c *MySQLClient) Close() error {
	return c.db.Close()
}

// TestConnection 测试数据库连接
func (c *MySQLClient) TestConnection() error {
	return c.db.Ping()
}

// GetNextID 获取下一个ID
func (c *MySQLClient) GetNextID() (string, error) {
	// 这里使用一个简单的方法生成ID，实际使用中可能需要更复杂的ID生成策略
	// 比如雪花算法等
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	return strconv.FormatInt(timestamp, 10), nil
}

// ClearDomainRecords 清除指定域名的现有记录（可选功能）
func (c *MySQLClient) ClearDomainRecords(domainID string) error {
	query := `DELETE FROM asset_sub_domain WHERE domain_id = ? AND source = 'Aliyun-DNS-Sync'`
	
	result, err := c.db.Exec(query, domainID)
	if err != nil {
		return fmt.Errorf("failed to clear domain records: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Cleared %d existing records for domain_id: %s", rowsAffected, domainID)
	
	return nil
}

// InsertSubDomains 批量插入子域名记录
func (c *MySQLClient) InsertSubDomains(records []*models.AssetSubDomain) error {
	if len(records) == 0 {
		return nil
	}

	// 开启事务
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 准备批量插入语句，使用INSERT IGNORE忽略重复记录
	query := `INSERT IGNORE INTO asset_sub_domain 
		(id, sub_domain, type, create_time, update_by, create_by, update_time, 
		 sys_org_code, dns_record, name_server, asset_label, asset_manager, 
		 asset_department, level, domain_id, source, project_id, aliyun_record_id) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	successCount := 0
	for _, record := range records {
		// 生成ID
		id, err := c.GetNextID()
		if err != nil {
			log.Printf("Failed to generate ID for record %s: %v", record.SubDomain, err)
			continue
		}
		record.ID = id

		// 执行插入
		_, err = stmt.Exec(
			record.ID,
			record.SubDomain,
			record.Type,
			record.CreateTime,
			record.UpdateBy,
			record.CreateBy,
			record.UpdateTime,
			record.SysOrgCode,
			record.DNSRecord,
			record.NameServer,
			record.AssetLabel,
			record.AssetManager,
			record.AssetDepartment,
			record.Level,
			record.DomainID,
			record.Source,
			record.ProjectID,
			record.AliyunRecordID,
		)

		if err != nil {
			log.Printf("Failed to insert record %s: %v", record.SubDomain, err)
			continue
		}
		
		successCount++
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully inserted %d/%d records", successCount, len(records))
	return nil
}

// CheckTableExists 检查表是否存在
func (c *MySQLClient) CheckTableExists() error {
	query := `SELECT COUNT(*) FROM information_schema.tables 
			  WHERE table_schema = DATABASE() AND table_name = 'asset_sub_domain'`
	
	var count int
	err := c.db.QueryRow(query).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("table 'asset_sub_domain' does not exist")
	}

	return nil
}

// GetLocalRecords 获取数据库中指定域名的所有记录
func (c *MySQLClient) GetLocalRecords(domainID string) (map[string]*models.AssetSubDomain, error) {
	query := `SELECT id, sub_domain, type, dns_record, aliyun_record_id, create_time, update_time
			  FROM asset_sub_domain 
			  WHERE domain_id = ? AND source = 'Aliyun-DNS-Sync' AND aliyun_record_id IS NOT NULL`
	
	rows, err := c.db.Query(query, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query local records: %w", err)
	}
	defer rows.Close()

	localRecords := make(map[string]*models.AssetSubDomain)
	
	for rows.Next() {
		record := &models.AssetSubDomain{}
		var aliyunRecordID sql.NullString
		var dnsRecord sql.NullString
		
		err := rows.Scan(
			&record.ID,
			&record.SubDomain,
			&record.Type,
			&dnsRecord,
			&aliyunRecordID,
			&record.CreateTime,
			&record.UpdateTime,
		)
		if err != nil {
			log.Printf("Failed to scan record: %v", err)
			continue
		}
		
		if aliyunRecordID.Valid {
			record.AliyunRecordID = &aliyunRecordID.String
			if dnsRecord.Valid {
				record.DNSRecord = &dnsRecord.String
			}
			localRecords[aliyunRecordID.String] = record
		}
	}
	
	return localRecords, nil
}

// InsertRecord 插入单条记录
func (c *MySQLClient) InsertRecord(record *models.AssetSubDomain) error {
	// 生成ID
	id, err := c.GetNextID()
	if err != nil {
		return fmt.Errorf("failed to generate ID: %w", err)
	}
	record.ID = id

	query := `INSERT INTO asset_sub_domain 
		(id, sub_domain, type, create_time, update_by, create_by, update_time, 
		 sys_org_code, dns_record, name_server, asset_label, asset_manager, 
		 asset_department, level, domain_id, source, project_id, aliyun_record_id) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = c.db.Exec(
		query,
		record.ID,
		record.SubDomain,
		record.Type,
		record.CreateTime,
		record.UpdateBy,
		record.CreateBy,
		record.UpdateTime,
		record.SysOrgCode,
		record.DNSRecord,
		record.NameServer,
		record.AssetLabel,
		record.AssetManager,
		record.AssetDepartment,
		record.Level,
		record.DomainID,
		record.Source,
		record.ProjectID,
		record.AliyunRecordID,
	)

	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}

	return nil
}

// UpdateRecord 更新记录
func (c *MySQLClient) UpdateRecord(localID string, aliyunRecord *models.DNSRecord) error {
	// 组合子域名
	subDomain := aliyunRecord.DomainName
	if aliyunRecord.RR != "" && aliyunRecord.RR != "@" {
		subDomain = aliyunRecord.RR + "." + aliyunRecord.DomainName
	}

	query := `UPDATE asset_sub_domain 
			  SET sub_domain = ?, type = ?, dns_record = ?, update_time = NOW() 
			  WHERE id = ?`

	_, err := c.db.Exec(query, subDomain, aliyunRecord.Type, aliyunRecord.Value, localID)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	return nil
}

// DeleteRecord 删除记录
func (c *MySQLClient) DeleteRecord(localID string) error {
	query := `DELETE FROM asset_sub_domain WHERE id = ?`
	
	_, err := c.db.Exec(query, localID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}

// NeedUpdate 检查记录是否需要更新
func NeedUpdate(aliyunRecord *models.DNSRecord, localRecord *models.AssetSubDomain) bool {
	// 组合阿里云记录的完整域名
	aliyunSubDomain := aliyunRecord.DomainName
	if aliyunRecord.RR != "" && aliyunRecord.RR != "@" {
		aliyunSubDomain = aliyunRecord.RR + "." + aliyunRecord.DomainName
	}

	// 比较关键字段
	if localRecord.SubDomain != aliyunSubDomain {
		return true
	}
	
	if localRecord.Type != aliyunRecord.Type {
		return true
	}
	
	if localRecord.DNSRecord == nil || *localRecord.DNSRecord != aliyunRecord.Value {
		return true
	}

	return false
}

// GetRecordCount 获取记录总数（用于统计）
func (c *MySQLClient) GetRecordCount(domainID string) (int, error) {
	query := `SELECT COUNT(*) FROM asset_sub_domain WHERE domain_id = ? AND source = 'Aliyun-DNS-Sync'`
	
	var count int
	err := c.db.QueryRow(query, domainID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get record count: %w", err)
	}

	return count, nil
}
