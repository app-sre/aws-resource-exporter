# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

AWS Resource Exporter is a Prometheus exporter for AWS resources, built in Go. It collects metadata and metrics from various AWS services (RDS, VPC, EC2, Route53, ElastiCache, IAM, MSK) to complement CloudWatch Exporter with resource information useful for Prometheus metrics.

## Development Commands

### Building
- `make build` - Build the binary locally
- `make image` - Build container image
- `make image-push` - Push container image to registry

### Testing
- `make test` - Run all tests (includes vet and unit tests)
- `make test-unit` - Run unit tests only
- `make vet` - Run go vet static analysis
- `make container-test` - Run tests in container

### Formatting
- `make format` - Format Go code
- `go fmt ./...` - Format specific packages

### Running Tests
- Individual test files follow the pattern `*_test.go`
- Tests use standard Go testing with stretchr/testify
- Mock files are generated in `pkg/awsclient/mock/`

## Architecture

### Core Components

**Main Entry Point (`main.go`)**
- Uses kingpin for CLI argument parsing
- Sets up Prometheus collectors for each AWS service
- Runs HTTP server on port 9115 (default) for `/metrics` endpoint

**Configuration (`pkg/config.go`)**
- YAML-based configuration with per-service settings
- Each service has BaseConfig with: enabled, interval, timeout, cache_ttl
- Service-specific configs extend BaseConfig (RDSConfig, VPCConfig, etc.)

**AWS Client Layer (`pkg/awsclient/`)**
- Centralized AWS SDK session management
- Service-specific client wrappers
- Mock interfaces for testing

**Service Collectors (`pkg/`)**
- Each AWS service has its own collector: `rds.go`, `vpc.go`, `ec2.go`, `route53.go`, `elasticache.go`, `iam.go`, `msk.go`
- Implement Prometheus collector interface
- Handle caching and rate limiting for AWS API calls

**Utilities (`pkg/`)**
- `cache.go` - TTL-based caching for AWS API responses
- `proxy.go` - HTTP proxy functionality
- `util.go` - Common utility functions

### Key Patterns

**Configuration Structure**
- Services are enabled/disabled via YAML config
- Default configuration file: `aws-resource-exporter-config.yaml`
- Environment variable override: `AWS_RESOURCE_EXPORTER_CONFIG_FILE`

**AWS Credentials**
- Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`
- Standard AWS SDK credential chain support

**Metrics Collection**
- Each service runs on configurable intervals
- Metrics are cached to prevent AWS API rate limits
- Parallel workers for intensive operations (RDS logs: `LOGS_METRICS_WORKERS`, `LOGS_METRICS_TTL`)

**Testing Strategy**
- Unit tests for each service collector
- Mock AWS clients using golang/mock
- Test files mirror source file structure (`pkg/*_test.go`)

## Exported Metrics

### RDS Metrics
- `aws_resources_exporter_rds_allocatedstorage` - The amount of allocated storage in GB
- `aws_resources_exporter_rds_dbinstanceclass` - The DB instance class (type)
- `aws_resources_exporter_rds_dbinstancestatus` - The instance status
- `aws_resources_exporter_rds_engineversion` - The DB engine type and version
- `aws_resources_exporter_rds_latest_restorable_time` - Latest restorable time timestamp
- `aws_resources_exporter_rds_max_connections` - Maximum connections for the instance
- `aws_resources_exporter_rds_max_connections_mapping_error` - Error mapping max connections
- `aws_resources_exporter_rds_pendingmaintenanceactions` - The pending maintenance actions for a RDS instance
- `aws_resources_exporter_rds_publicly_accessible` - Whether the instance is publicly accessible
- `aws_resources_exporter_rds_storage_encrypted` - Whether storage is encrypted
- `aws_resources_exporter_rds_logs_amount` - The amount of log files present in the RDS Instance
- `aws_resources_exporter_rds_logsstorage_size_bytes` - The amount of storage used by the log files
- `aws_resources_exporter_rds_eol_infos` - End of life information for RDS engines

### VPC Metrics
- `aws_resources_exporter_vpc_vpcsperregion_quota` - Quota for VPCs per region
- `aws_resources_exporter_vpc_vpcsperregion_usage` - Usage of VPCs per region
- `aws_resources_exporter_vpc_subnetspervpc_quota` - Quota for subnets per VPC
- `aws_resources_exporter_vpc_subnetspervpc_usage` - Usage of subnets per VPC
- `aws_resources_exporter_vpc_interfacevpcendpointspervpc_quota` - Quota for interface endpoints per VPC
- `aws_resources_exporter_vpc_interfacevpcendpointspervpc_usage` - Usage of interface endpoints per VPC
- `aws_resources_exporter_vpc_routetablespervpc_quota` - Quota for route tables per VPC
- `aws_resources_exporter_vpc_routetablespervpc_usage` - Usage of route tables per VPC
- `aws_resources_exporter_vpc_routesperroutetable_quota` - Quota for routes per route table
- `aws_resources_exporter_vpc_routesperroutetable_usage` - Usage of routes per route table
- `aws_resources_exporter_vpc_ipv4blockspervpc_quota` - Quota for IPv4 blocks per VPC
- `aws_resources_exporter_vpc_ipv4blockspervpc_usage` - Usage of IPv4 blocks per VPC

### EC2 Metrics
- `aws_resources_exporter_ec2_transitgatewaysperregion_quota` - Quota for transit gateways per region
- `aws_resources_exporter_ec2_transitgatewaysperregion_usage` - Usage of transit gateways per region

### Route53 Metrics
- `aws_resources_exporter_route53_recordsperhostedzone_quota` - Quota for records per hosted zone
- `aws_resources_exporter_route53_recordsperhostedzone_total` - Number of resource records per hosted zone
- `aws_resources_exporter_route53_hostedzonesperaccount_quota` - Quota for hosted zones per account
- `aws_resources_exporter_route53_hostedzonesperaccount_total` - Number of hosted zones in account
- `aws_resources_exporter_route53_last_updated_timestamp_seconds` - Last update timestamp

### IAM Metrics
- `aws_resources_exporter_iam_roles_used` - Number of IAM roles in use
- `aws_resources_exporter_iam_roles_quota` - Quota for IAM roles

### ElastiCache Metrics
- `aws_resources_exporter_elasticache_redis_version` - Redis version information

### MSK Metrics
- `aws_resources_exporter_msk_info` - MSK cluster information

### AWS Client Metrics
- `aws_resources_exporter_awsclient_api_requests_total` - Total AWS API requests made
- `aws_resources_exporter_awsclient_api_errors_total` - Total AWS API errors encountered
