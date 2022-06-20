# aws-resource-exporter

Prometheus exporter for AWS resources

This was made as a complement to [CloudWatch Exporter](https://github.com/prometheus/cloudwatch_exporter) to get resource information that are useful to keep around as metrics in Prometheus but are out of scope for CloudWatch Exporter.

## Included metadata & metrics

| Service | Metric                    | Description                                         |
|---------|---------------------------|-----------------------------------------------------|
| RDS     | allocatedstorage          | The amount of allocated storage in GB               |
| RDS     | dbinstanceclass           | The DB instance class (type)                        |
| RDS     | dbinstancestatus          | The instance status                                 |
| RDS     | engineversion             | The DB engine type and version                      |
| RDS     | pendingmaintenanceactions | The pending maintenance actions for a RDS instance  |
| RDS     | logs_amount               | The amount of log files present in the RDS Instance |
| RDS     | logsstorage_size_bytes    | The amount of storage used by the log files nstance |


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

AWS credentials can be passed as environment variables `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. AWS region must be passed via `AWS_REGION`.

RDS Logs metrics are requested in parallel to improve the scrappping time. Also, metrics are cached to prevent AWS api rate limits. Parameters to
tweak this behavior.

- `LOGS_METRICS_WORKERS`: Number of workers to request log metrics in parallel (default=10)
- `LOGS_METRICS_TTL`: Cache TTL for rds logs related metrics (default=300)

To view all available command-line flags, run `./aws-resource-exporter -h`.

## License

Apache License 2.0, see [LICENSE](LICENSE).
