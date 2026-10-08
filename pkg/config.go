package pkg

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type BaseConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Interval *time.Duration `yaml:"interval"`
	Timeout  *time.Duration `yaml:"timeout"`
	CacheTTL *time.Duration `yaml:"cache_ttl"`
}

type RDSConfig struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string    `yaml:"regions"`
	Thresholds []Threshold `yaml:"thresholds"`
	// DeprecatedEOLInfos only exists so LoadExporterConfiguration can detect and warn about a
	// stale eol_info block left over in config -- RDS EOL data is now resolved live from AWS
	// and no longer reads this. yaml.Unmarshal silently ignores unknown keys, so without this
	// field a leftover eol_info block in app-interface's configmap would go unnoticed.
	DeprecatedEOLInfos []EOLInfo `yaml:"eol_info"`
}
type Threshold struct {
	Name string `yaml:"name"`
	Days int    `yaml:"days"`
}

type EOLInfo struct {
	Engine  string `yaml:"engine"`
	EOL     string `yaml:"eol"`
	Version string `yaml:"version"`
}

type EOLKey struct {
	Engine  string
	Version string
}

type VPCConfig struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string `yaml:"regions"`
}

type Route53Config struct {
	BaseConfig    `yaml:"base,inline"`
	Region        string `yaml:"region"` // Use only a single Region for now, as the current metric is global
	SkipZoneLimit bool   `yaml:"skip_zone_limit"`
}

type EC2Config struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string `yaml:"regions"`
}

type ElastiCacheConfig struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string    `yaml:"regions"`
	EOLInfos   []EOLInfo   `yaml:"eol_info"`
	Thresholds []Threshold `yaml:"thresholds"`
}
type MSKConfig struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string    `yaml:"regions"`
	MSKInfos   []MSKInfo   `yaml:"msk_info"`
	Thresholds []Threshold `yaml:"thresholds"`
}

type MSKInfo struct {
	EOL     string `yaml:"eol"`
	Version string `yaml:"version"`
}

type IAMConfig struct {
	BaseConfig `yaml:"base,inline"`
	Region     string `yaml:"region"`
}

type Config struct {
	RdsConfig         RDSConfig         `yaml:"rds"`
	VpcConfig         VPCConfig         `yaml:"vpc"`
	Route53Config     Route53Config     `yaml:"route53"`
	EC2Config         EC2Config         `yaml:"ec2"`
	ElastiCacheConfig ElastiCacheConfig `yaml:"elasticache"`
	MskConfig         MSKConfig         `yaml:"msk"`
	IamConfig         IAMConfig         `yaml:"iam"`
}

func LoadExporterConfiguration(logger *slog.Logger, configFile string) (*Config, error) {
	var config Config
	file, err := os.ReadFile(configFile)
	if err != nil {
		logger.Error("Could not load configuration file")
		return nil, errors.New("Could not load configuration file: " + configFile)
	}
	yaml.Unmarshal(file, &config)

	if len(config.RdsConfig.DeprecatedEOLInfos) > 0 {
		logger.Warn("rds.eol_info is set but no longer used -- RDS EOL data is now resolved live from AWS. Remove this block from the config.")
	}

	if config.RdsConfig.CacheTTL == nil {
		config.RdsConfig.CacheTTL = durationPtr(35 * time.Second)
	}
	if config.VpcConfig.CacheTTL == nil {
		config.VpcConfig.CacheTTL = durationPtr(35 * time.Second)
	}
	if config.Route53Config.CacheTTL == nil {
		config.Route53Config.CacheTTL = durationPtr(35 * time.Second)
	}
	if config.EC2Config.CacheTTL == nil {
		config.EC2Config.CacheTTL = durationPtr(35 * time.Second)
	}
	if config.ElastiCacheConfig.CacheTTL == nil {
		config.ElastiCacheConfig.CacheTTL = durationPtr(35 * time.Second)
	}
	if config.MskConfig.CacheTTL == nil {
		config.MskConfig.CacheTTL = durationPtr(35 * time.Second)
	}
	if config.IamConfig.CacheTTL == nil {
		config.IamConfig.CacheTTL = durationPtr(35 * time.Second)
	}

	if config.RdsConfig.Interval == nil {
		config.RdsConfig.Interval = durationPtr(15 * time.Second)
	}
	if config.VpcConfig.Interval == nil {
		config.VpcConfig.Interval = durationPtr(15 * time.Second)
	}
	if config.Route53Config.Interval == nil {
		config.Route53Config.Interval = durationPtr(15 * time.Second)
	}
	if config.EC2Config.Interval == nil {
		config.EC2Config.Interval = durationPtr(15 * time.Second)
	}
	if config.ElastiCacheConfig.Interval == nil {
		config.ElastiCacheConfig.Interval = durationPtr(15 * time.Second)
	}
	if config.MskConfig.Interval == nil {
		config.MskConfig.Interval = durationPtr(15 * time.Second)
	}
	if config.IamConfig.Interval == nil {
		config.IamConfig.Interval = durationPtr(15 * time.Second)
	}

	if config.RdsConfig.Timeout == nil {
		config.RdsConfig.Timeout = durationPtr(10 * time.Second)
	}
	if config.VpcConfig.Timeout == nil {
		config.VpcConfig.Timeout = durationPtr(10 * time.Second)
	}
	if config.Route53Config.Timeout == nil {
		config.Route53Config.Timeout = durationPtr(10 * time.Second)
	}
	if config.EC2Config.Timeout == nil {
		config.EC2Config.Timeout = durationPtr(10 * time.Second)
	}
	if config.ElastiCacheConfig.Timeout == nil {
		config.ElastiCacheConfig.Timeout = durationPtr(10 * time.Second)
	}
	if config.MskConfig.Timeout == nil {
		config.MskConfig.Timeout = durationPtr(10 * time.Second)
	}
	if config.IamConfig.Timeout == nil {
		config.IamConfig.Timeout = durationPtr(10 * time.Second)
	}

	// Setting defaults when threshold is not defined to ease the transition from hardcoded thresholds
	if len(config.RdsConfig.Thresholds) == 0 {
		config.RdsConfig.Thresholds = []Threshold{
			{Name: "red", Days: 90},
			{Name: "yellow", Days: 180},
			{Name: "green", Days: 365},
		}
	}
	if len(config.ElastiCacheConfig.Thresholds) == 0 {
		config.ElastiCacheConfig.Thresholds = []Threshold{
			{Name: "red", Days: 90},
			{Name: "yellow", Days: 180},
			{Name: "green", Days: 365},
		}
	}

	return &config, nil
}
