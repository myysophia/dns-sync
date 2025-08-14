package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dns-sync/internal/aliyun"
	"dns-sync/internal/config"
	"dns-sync/internal/database"
	"dns-sync/internal/models"
)

// SyncStats 同步统计信息
type SyncStats struct {
	Domain  string
	Added   int
	Updated int
	Deleted int
	Error   string
}

func main() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Starting DNS incremental sync application...")

	// 加载配置文件
	configPath := filepath.Join("config", "config.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("Configuration loaded successfully")

	// 初始化阿里云DNS客户端
	dnsClient, err := aliyun.NewDNSClient(&cfg.Aliyun)
	if err != nil {
		log.Fatalf("Failed to create DNS client: %v", err)
	}
	log.Println("Aliyun DNS client initialized")

	// 测试阿里云连接
	if err := dnsClient.TestConnection(); err != nil {
		log.Fatalf("Failed to test Aliyun connection: %v", err)
	}
	log.Println("Aliyun connection test passed")

	// 初始化MySQL客户端
	mysqlClient, err := database.NewMySQLClient(&cfg.MySQL)
	if err != nil {
		log.Fatalf("Failed to create MySQL client: %v", err)
	}
	defer mysqlClient.Close()
	log.Println("MySQL client initialized")

	// 测试数据库连接
	if err := mysqlClient.TestConnection(); err != nil {
		log.Fatalf("Failed to test MySQL connection: %v", err)
	}
	log.Println("MySQL connection test passed")

	// 检查数据库表是否存在
	if err := mysqlClient.CheckTableExists(); err != nil {
		log.Fatalf("Database table check failed: %v", err)
	}
	log.Println("Database table exists")

	// 执行增量同步
	var syncStats []*SyncStats
	totalAdded := 0
	totalUpdated := 0
	totalDeleted := 0

	for _, domainMapping := range cfg.Domains {
		log.Printf("Processing domain: %s (project_id: %s, domain_id: %s)",
			domainMapping.Domain, domainMapping.ProjectID, domainMapping.DomainID)

		stats := &SyncStats{
			Domain: domainMapping.Domain,
		}

		// 执行单个域名的增量同步
		added, updated, deleted, err := incrementalSyncDomain(dnsClient, mysqlClient, domainMapping)
		if err != nil {
			stats.Error = err.Error()
			log.Printf("Error syncing domain %s: %v", domainMapping.Domain, err)
		} else {
			stats.Added = added
			stats.Updated = updated
			stats.Deleted = deleted
			totalAdded += added
			totalUpdated += updated
			totalDeleted += deleted
			log.Printf("Domain %s sync completed: +%d ~%d -%d", 
				domainMapping.Domain, added, updated, deleted)
		}

		syncStats = append(syncStats, stats)
	}

	// 打印同步结果摘要
	printIncrementalSyncSummary(syncStats, totalAdded, totalUpdated, totalDeleted)

	log.Println("DNS incremental sync application completed")
}

// incrementalSyncDomain 执行单个域名的增量同步
func incrementalSyncDomain(dnsClient *aliyun.DNSClient, mysqlClient *database.MySQLClient, 
	domainMapping config.DomainMapping) (int, int, int, error) {
	
	// 1. 获取阿里云当前所有DNS记录
	dnsRecords, err := dnsClient.GetDomainRecords(domainMapping.Domain)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get DNS records: %w", err)
	}

	// 2. 过滤只处理A和CNAME记录，且状态为ENABLE
	var validRecords []*models.DNSRecord
	for _, record := range dnsRecords {
		if (record.Type == "A" || record.Type == "CNAME") && record.Status == "ENABLE" {
			validRecords = append(validRecords, record)
		}
	}

	log.Printf("Found %d valid DNS records (A/CNAME, ENABLED) for domain: %s", 
		len(validRecords), domainMapping.Domain)

	// 3. 获取数据库中该域名的所有记录
	localRecords, err := mysqlClient.GetLocalRecords(domainMapping.DomainID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get local records: %w", err)
	}

	log.Printf("Found %d local records for domain: %s", len(localRecords), domainMapping.Domain)

	// 4. 构建阿里云记录映射表
	aliyunRecords := make(map[string]*models.DNSRecord)
	for _, record := range validRecords {
		aliyunRecords[record.RecordId] = record
	}

	// 5. 执行三向对比同步
	added := 0
	updated := 0
	deleted := 0

	// 处理新增和更新
	for recordId, aliyunRecord := range aliyunRecords {
		if localRecord, exists := localRecords[recordId]; exists {
			// 记录存在，检查是否需要更新
			if database.NeedUpdate(aliyunRecord, localRecord) {
				err := mysqlClient.UpdateRecord(localRecord.ID, aliyunRecord)
				if err != nil {
					log.Printf("Failed to update record %s: %v", recordId, err)
				} else {
					updated++
					log.Printf("Updated record: %s -> %s", localRecord.SubDomain, 
						getFullDomain(aliyunRecord))
				}
			}
		} else {
			// 新记录，插入数据库
			newRecord := aliyunRecord.ConvertToAssetSubDomain(
				domainMapping.DomainID, 
				domainMapping.ProjectID,
			)
			err := mysqlClient.InsertRecord(newRecord)
			if err != nil {
				log.Printf("Failed to insert record %s: %v", recordId, err)
			} else {
				added++
				log.Printf("Added new record: %s", newRecord.SubDomain)
			}
		}
	}

	// 处理删除
	for recordId, localRecord := range localRecords {
		if _, exists := aliyunRecords[recordId]; !exists {
			// 阿里云已删除，数据库也删除
			err := mysqlClient.DeleteRecord(localRecord.ID)
			if err != nil {
				log.Printf("Failed to delete record %s: %v", recordId, err)
			} else {
				deleted++
				log.Printf("Deleted record: %s", localRecord.SubDomain)
			}
		}
	}

	return added, updated, deleted, nil
}

// getFullDomain 获取完整域名
func getFullDomain(record *models.DNSRecord) string {
	if record.RR == "" || record.RR == "@" {
		return record.DomainName
	}
	return record.RR + "." + record.DomainName
}

// printIncrementalSyncSummary 打印增量同步结果摘要
func printIncrementalSyncSummary(stats []*SyncStats, totalAdded, totalUpdated, totalDeleted int) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DNS INCREMENTAL SYNC SUMMARY")
	fmt.Println(strings.Repeat("=", 70))

	successCount := 0
	failureCount := 0

	for _, stat := range stats {
		if stat.Error != "" {
			fmt.Printf("%-20s ✗ FAILED\n", stat.Domain)
			fmt.Printf("  Error: %s\n", stat.Error)
			failureCount++
		} else {
			fmt.Printf("%-20s ✓ SUCCESS (+%d ~%d -%d)\n", 
				stat.Domain, stat.Added, stat.Updated, stat.Deleted)
			successCount++
		}
	}

	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Total domains processed: %d\n", len(stats))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failureCount)
	fmt.Printf("Total changes: +%d ~%d -%d\n", totalAdded, totalUpdated, totalDeleted)
	fmt.Printf("Sync time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 70))

	// 如果有失败的同步，退出时返回错误代码
	if failureCount > 0 {
		os.Exit(1)
	}
}
