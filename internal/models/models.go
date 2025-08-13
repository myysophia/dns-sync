package models

import (
	"time"
)

// DNSRecord 阿里云DNS记录结构体
type DNSRecord struct {
	CreateTimestamp int64  `json:"CreateTimestamp"`
	DomainName      string `json:"DomainName"`
	LbaStatus       bool   `json:"LbaStatus"`
	Line            string `json:"Line"`
	Locked          bool   `json:"Locked"`
	RR              string `json:"RR"`
	RecordId        string `json:"RecordId"`
	Status          string `json:"Status"`
	TTL             int32  `json:"TTL"`
	Type            string `json:"Type"`
	UpdateTimestamp int64  `json:"UpdateTimestamp"`
	Value           string `json:"Value"`
	Weight          int32  `json:"Weight"`
}

// AssetSubDomain 数据库中的子域名记录
type AssetSubDomain struct {
	ID               string     `db:"id"`
	SubDomain        string     `db:"sub_domain"`
	Type             string     `db:"type"`
	CreateTime       time.Time  `db:"create_time"`
	UpdateBy         *string    `db:"update_by"`
	CreateBy         *string    `db:"create_by"`
	UpdateTime       time.Time  `db:"update_time"`
	SysOrgCode       *string    `db:"sys_org_code"`
	DNSRecord        *string    `db:"dns_record"`
	NameServer       *string    `db:"name_server"`
	AssetLabel       string     `db:"asset_label"`
	AssetManager     *string    `db:"asset_manager"`
	AssetDepartment  *string    `db:"asset_department"`
	Level            *string    `db:"level"`
	DomainID         string     `db:"domain_id"`
	Source           string     `db:"source"`
	ProjectID        string     `db:"project_id"`
}

// ConvertToAssetSubDomain 将阿里云DNS记录转换为数据库记录
func (d *DNSRecord) ConvertToAssetSubDomain(domainID, projectID string) *AssetSubDomain {
	now := time.Now()
	
	// 组合子域名：如果RR为空或为@，则使用域名本身，否则拼接RR和域名
	subDomain := d.DomainName
	if d.RR != "" && d.RR != "@" {
		subDomain = d.RR + "." + d.DomainName
	}

	return &AssetSubDomain{
		SubDomain:       subDomain,
		Type:            d.Type,
		CreateTime:      now,
		UpdateTime:      now,
		AssetLabel:      "",
		DomainID:        domainID,
		Source:          "Aliyun-DNS-Sync",
		ProjectID:       projectID,
	}
}

// DomainSyncResult 同步结果
type DomainSyncResult struct {
	Domain      string `json:"domain"`
	Success     bool   `json:"success"`
	RecordCount int    `json:"record_count"`
	Error       string `json:"error,omitempty"`
}
