package pkg

import (
	"errors"
	"io/ioutil"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"gopkg.in/yaml.v2"
)

type BaseConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Interval *time.Duration `yaml:"interval"`
	Timeout  *time.Duration `yaml:"timeout"`
	CacheTTL *time.Duration `yaml:"cache_ttl"`
}

type RDSConfig struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string `yaml:"regions"`
}

type VPCConfig struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string `yaml:"regions"`
}

type Route53Config struct {
	BaseConfig `yaml:"base,inline"`
	Region     string `yaml:"region"` // Use only a single Region for now, as the current metric is global
}

type EC2Config struct {
	BaseConfig `yaml:"base,inline"`
	Regions    []string `yaml:"regions"`
}

type Config struct {
	RdsConfig     RDSConfig     `yaml:"rds"`
	VpcConfig     VPCConfig     `yaml:"vpc"`
	Route53Config Route53Config `yaml:"route53"`
	EC2Config     EC2Config     `yaml:"ec2"`
}

func LoadExporterConfiguration(logger log.Logger, configFile string) (*Config, error) {
	var config Config
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		level.Error(logger).Log("Could not load configuration file")
		return nil, errors.New("Could not load configuration file: " + configFile)
	}
	yaml.Unmarshal(file, &config)

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
	return &config, nil
}
