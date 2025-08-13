package aliyun

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"dns-sync/internal/config"
	"dns-sync/internal/models"
)

// DNSClient 阿里云DNS客户端
type DNSClient struct {
	accessKeyID     string
	accessKeySecret string
	region          string
	endpoint        string
}

// DomainRecordsResponse API响应结构
type DomainRecordsResponse struct {
	TotalCount    int64 `json:"TotalCount"`
	PageNumber    int64 `json:"PageNumber"`
	PageSize      int64 `json:"PageSize"`
	RequestId     string `json:"RequestId"`
	DomainRecords struct {
		Record []struct {
			DomainName      string `json:"DomainName"`
			RecordId        string `json:"RecordId"`
			RR              string `json:"RR"`
			Type            string `json:"Type"`
			Value           string `json:"Value"`
			Line            string `json:"Line"`
			Priority        *int32 `json:"Priority,omitempty"`
			TTL             *int32 `json:"TTL,omitempty"`
			Status          string `json:"Status"`
			Locked          bool   `json:"Locked"`
			Weight          *int32 `json:"Weight,omitempty"`
			CreateTimestamp *int64 `json:"CreateTimestamp,omitempty"`
			UpdateTimestamp *int64 `json:"UpdateTimestamp,omitempty"`
		} `json:"Record"`
	} `json:"DomainRecords"`
}

// DomainsResponse API响应结构用于测试连接
type DomainsResponse struct {
	TotalCount int64 `json:"TotalCount"`
	PageNumber int64 `json:"PageNumber"`
	PageSize   int64 `json:"PageSize"`
	RequestId  string `json:"RequestId"`
}

// NewDNSClient 创建DNS客户端
func NewDNSClient(cfg *config.AliyunConfig) (*DNSClient, error) {
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("access key id and secret are required")
	}

	endpoint := "https://alidns.cn-hangzhou.aliyuncs.com"
	if cfg.Region != "" && cfg.Region != "cn-hangzhou" {
		endpoint = fmt.Sprintf("https://alidns.%s.aliyuncs.com", cfg.Region)
	}

	return &DNSClient{
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		region:          cfg.Region,
		endpoint:        endpoint,
	}, nil
}

// signRequest 对请求进行签名
func (c *DNSClient) signRequest(params map[string]string) string {
	// 添加公共参数
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["AccessKeyId"] = c.accessKeyID
	params["SignatureMethod"] = "HMAC-SHA1"
	params["Timestamp"] = timestamp
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = strconv.FormatInt(time.Now().UnixNano(), 10)
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"

	// 排序参数
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建查询字符串
	var sortedParams []string
	for _, k := range keys {
		sortedParams = append(sortedParams, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	queryString := strings.Join(sortedParams, "&")

	// 构建待签名字符串
	stringToSign := "GET&%2F&" + url.QueryEscape(queryString)

	// 计算签名
	mac := hmac.New(sha1.New, []byte(c.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// makeRequest 发送HTTP请求
func (c *DNSClient) makeRequest(params map[string]string) ([]byte, error) {
	signature := c.signRequest(params)
	params["Signature"] = signature

	// 构建URL
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	query := u.Query()
	for k, v := range params {
		query.Set(k, v)
	}
	u.RawQuery = query.Encode()

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetDomainRecords 获取域名的DNS记录
func (c *DNSClient) GetDomainRecords(domain string) ([]*models.DNSRecord, error) {
	log.Printf("Getting DNS records for domain: %s", domain)

	var allRecords []*models.DNSRecord
	pageNumber := int64(1)
	pageSize := int64(100)

	for {
		params := map[string]string{
			"Action":     "DescribeDomainRecords",
			"DomainName": domain,
			"PageNumber": strconv.FormatInt(pageNumber, 10),
			"PageSize":   strconv.FormatInt(pageSize, 10),
		}

		body, err := c.makeRequest(params)
		if err != nil {
			return nil, fmt.Errorf("failed to describe domain records for %s: %w", domain, err)
		}

		var response DomainRecordsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// 转换记录格式
		for _, record := range response.DomainRecords.Record {
			dnsRecord := &models.DNSRecord{
				DomainName: record.DomainName,
				RR:         record.RR,
				RecordId:   record.RecordId,
				Type:       record.Type,
				Value:      record.Value,
				Line:       record.Line,
				Status:     record.Status,
				Locked:     record.Locked,
			}

			// 处理可能为nil的字段
			if record.TTL != nil {
				dnsRecord.TTL = *record.TTL
			}
			if record.Weight != nil {
				dnsRecord.Weight = *record.Weight
			}
			if record.CreateTimestamp != nil {
				dnsRecord.CreateTimestamp = *record.CreateTimestamp
			}
			if record.UpdateTimestamp != nil {
				dnsRecord.UpdateTimestamp = *record.UpdateTimestamp
			}

			allRecords = append(allRecords, dnsRecord)
		}

		// 检查是否还有更多页面
		if int64(len(allRecords)) >= response.TotalCount {
			break
		}

		pageNumber++
		if pageNumber > (response.TotalCount/pageSize)+1 {
			break
		}
	}

	log.Printf("Retrieved %d DNS records for domain: %s", len(allRecords), domain)
	return allRecords, nil
}

// TestConnection 测试连接
func (c *DNSClient) TestConnection() error {
	log.Println("Testing Aliyun DNS connection...")

	params := map[string]string{
		"Action":     "DescribeDomains",
		"PageNumber": "1",
		"PageSize":   "1",
	}

	body, err := c.makeRequest(params)
	if err != nil {
		return fmt.Errorf("failed to test aliyun connection: %w", err)
	}

	var response DomainsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse test response: %w", err)
	}

	log.Println("Aliyun DNS connection test successful")
	return nil
}
