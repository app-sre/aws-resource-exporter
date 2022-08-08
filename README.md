# aws-resource-exporter

Prometheus exporter for AWS resources

This was made as a complement to [CloudWatch Exporter](https://github.com/prometheus/cloudwatch_exporter) to get resource information that are useful to keep around as metrics in Prometheus but are out of scope for CloudWatch Exporter.

## Included metadata & metrics

| Service | Metric                      | Description                                         |
|---------|-----------------------------|-----------------------------------------------------|
| RDS     | allocatedstorage            | The amount of allocated storage in GB               |
| RDS     | dbinstanceclass             | The DB instance class (type)                        |
| RDS     | dbinstancestatus            | The instance status                                 |
| RDS     | engineversion               | The DB engine type and version                      |
| RDS     | pendingmaintenanceactions   | The pending maintenance actions for a RDS instance  |
| RDS     | logs_amount                 | The amount of log files present in the RDS Instance |
| RDS     | logsstorage_size_bytes      | The amount of storage used by the log files nstance |
| VPC     | vpcsperregion               | Quota and usage of the VPCs per region              |
| VPC     | subnetspervpc               | Quota and usage of subnets per VPC                  |
| VPC     | interfacevpcendpointspervpc | Quota and usage of interface endpoints per VPC      |
| VPC     | routetablespervpc           | Quota and usage of routetables per VPC              |
| VPC     | routesperroutetable         | Quota and usage of the routes per routetable        |
| VPC     | ipv4blockspervpc            | Quota and usage of ipv4 blocks per VPC              |
| Route53 | recordsperhostedzone        | Quota and usage of resource records per Hosted Zone |


## Running this software

### From binaries

Download the most suitable binary from [the releases tab](https://github.com/app-sre/aws-resource-exporter/releases)

Then:

    ./aws-resource-exporter <flags>

### Using the container image

    docker run --rm -d -p 9115:9115 \
        --name aws-resource-exporter \
        --env AWS_ACCESS_KEY_ID=AAA \
        --env AWS_SECRET_ACCESS_KEY=AAA \
        --env AWS_REGION=AAA \
        quay.io/app-sre/aws-resource-exporter:latest

## Building the software

### Local Build

    make build

### Building docker image

    make image image-push

## Configuration

AWS credentials can be passed as environment variables `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.

Additional configuration can be supplied in a configuration file and might differ between collectors.

An example file can look like this:

```yaml
rds:
  enabled: true
  regions: "us-east-1"
vpc:
  enabled: true
  regions: "us-east-1,eu-central-1"
  timeout: 30s
route53:
  enabled: true
  regions: "us-east1"
  timeout: 60s
```

Some exporters might expose different configuration values, see the example files for possible keys.

The config file location can be specified using the environment variable `AWS_RESOURCE_EXPORTER_CONFIG_FILE`.

RDS Logs metrics are requested in parallel to improve the scrappping time. Also, metrics are cached to prevent AWS api rate limits. Parameters to
tweak this behavior.

- `LOGS_METRICS_WORKERS`: Number of workers to request log metrics in parallel (default=10)
- `LOGS_METRICS_TTL`: Cache TTL for rds logs related metrics (default=300)

To view all available command-line flags, run `./aws-resource-exporter -h`.

## License

Apache License 2.0, see [LICENSE](LICENSE).
