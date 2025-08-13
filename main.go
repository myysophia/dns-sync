package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"dns-sync/internal/aliyun"
	"dns-sync/internal/config"
	"dns-sync/internal/database"
	"dns-sync/internal/models"
)

func main() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	log.Println("Starting DNS sync application...")

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

	// 同步每个域名的DNS记录
	var syncResults []*models.DomainSyncResult
	totalRecords := 0

	for _, domainMapping := range cfg.Domains {
		log.Printf("Processing domain: %s (project_id: %s, domain_id: %s)",
			domainMapping.Domain, domainMapping.ProjectID, domainMapping.DomainID)

		result := &models.DomainSyncResult{
			Domain:  domainMapping.Domain,
			Success: false,
		}

		// 获取DNS记录
		dnsRecords, err := dnsClient.GetDomainRecords(domainMapping.Domain)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to get DNS records: %v", err)
			log.Printf("Error getting DNS records for %s: %v", domainMapping.Domain, err)
			syncResults = append(syncResults, result)
			continue
		}

		if len(dnsRecords) == 0 {
			log.Printf("No DNS records found for domain: %s", domainMapping.Domain)
			result.Success = true
			result.RecordCount = 0
			syncResults = append(syncResults, result)
			continue
		}

		log.Printf("Found %d DNS records for domain: %s", len(dnsRecords), domainMapping.Domain)

		// 转换为数据库记录格式，只同步A记录和CNAME记录
		var subDomainRecords []*models.AssetSubDomain
		validRecordCount := 0
		for _, dnsRecord := range dnsRecords {
			// 只处理A记录和CNAME记录
			if dnsRecord.Type == "A" || dnsRecord.Type == "CNAME" {
				subDomainRecord := dnsRecord.ConvertToAssetSubDomain(
					domainMapping.DomainID,
					domainMapping.ProjectID,
				)
				subDomainRecords = append(subDomainRecords, subDomainRecord)
				validRecordCount++
			}
		}

		log.Printf("Filtered %d A/CNAME records from %d total records for domain: %s", 
			validRecordCount, len(dnsRecords), domainMapping.Domain)

		// 可选：清除该域名的现有记录（避免重复）
		// 如果需要增量更新而不是全量替换，请注释掉下面这行
		if err := mysqlClient.ClearDomainRecords(domainMapping.DomainID); err != nil {
			log.Printf("Warning: Failed to clear existing records for %s: %v", domainMapping.Domain, err)
		}

		// 批量插入数据库
		if err := mysqlClient.InsertSubDomains(subDomainRecords); err != nil {
			result.Error = fmt.Sprintf("Failed to insert records: %v", err)
			log.Printf("Error inserting records for %s: %v", domainMapping.Domain, err)
			syncResults = append(syncResults, result)
			continue
		}

		// 验证插入结果
		recordCount, err := mysqlClient.GetRecordCount(domainMapping.DomainID)
		if err != nil {
			log.Printf("Warning: Failed to get record count for %s: %v", domainMapping.Domain, err)
		}

		result.Success = true
		result.RecordCount = len(subDomainRecords)
		totalRecords += result.RecordCount
		syncResults = append(syncResults, result)

		log.Printf("Successfully synced %d records for domain: %s (DB count: %d)",
			result.RecordCount, domainMapping.Domain, recordCount)
	}

	// 打印同步结果摘要
	printSyncSummary(syncResults, totalRecords)

	log.Println("DNS sync application completed")
}

// printSyncSummary 打印同步结果摘要
func printSyncSummary(results []*models.DomainSyncResult, totalRecords int) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DNS SYNC SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	successCount := 0
	failureCount := 0

	for _, result := range results {
		status := "✓ SUCCESS"
		if !result.Success {
			status = "✗ FAILED"
			failureCount++
		} else {
			successCount++
		}

		fmt.Printf("%-20s %s (Records: %d)\n", result.Domain, status, result.RecordCount)
		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Total domains processed: %d\n", len(results))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failureCount)
	fmt.Printf("Total DNS records synced: %d\n", totalRecords)
	fmt.Println(strings.Repeat("=", 60))

	// 如果有失败的同步，退出时返回错误代码
	if failureCount > 0 {
		os.Exit(1)
	}
}
