package config

import (
	"fmt"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

// AliyunConfig 阿里云配置
type AliyunConfig struct {
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Region          string `yaml:"region"`
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// DomainMapping 域名映射关系
type DomainMapping struct {
	ProjectID string `yaml:"project_id"`
	DomainID  string `yaml:"domain_id"`
	Domain    string `yaml:"domain"`
}

// Config 应用配置
type Config struct {
	Aliyun  AliyunConfig    `yaml:"aliyun"`
	MySQL   MySQLConfig     `yaml:"mysql"`
	Domains []DomainMapping `yaml:"domains"`
}

// LoadConfig 加载配置文件
func LoadConfig(filepath string) (*Config, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// validate 验证配置的完整性
func (c *Config) validate() error {
	if c.Aliyun.AccessKeyID == "" {
		return fmt.Errorf("aliyun access_key_id is required")
	}
	if c.Aliyun.AccessKeySecret == "" {
		return fmt.Errorf("aliyun access_key_secret is required")
	}
	if c.MySQL.Host == "" {
		return fmt.Errorf("mysql host is required")
	}
	if c.MySQL.Username == "" {
		return fmt.Errorf("mysql username is required")
	}
	if c.MySQL.Database == "" {
		return fmt.Errorf("mysql database is required")
	}
	if len(c.Domains) == 0 {
		return fmt.Errorf("at least one domain mapping is required")
	}

	for i, domain := range c.Domains {
		if domain.ProjectID == "" || domain.DomainID == "" || domain.Domain == "" {
			return fmt.Errorf("invalid domain mapping at index %d", i)
		}
	}

	return nil
}

// GetMySQLDSN 获取MySQL连接字符串
func (c *Config) GetMySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.Username, c.MySQL.Password, c.MySQL.Host, c.MySQL.Port, c.MySQL.Database)
}
