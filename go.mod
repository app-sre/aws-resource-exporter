module github.com/app-sre/aws-resource-exporter

go 1.23.0

toolchain go1.23.11

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/aws/aws-sdk-go-v2 v1.36.6
	github.com/aws/aws-sdk-go-v2/config v1.29.18
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.235.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.46.4
	github.com/aws/aws-sdk-go-v2/service/iam v1.43.1
	github.com/aws/aws-sdk-go-v2/service/kafka v1.39.7
	github.com/aws/aws-sdk-go-v2/service/rds v1.99.2
	github.com/aws/aws-sdk-go-v2/service/route53 v1.53.1
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.28.4
	github.com/aws/aws-sdk-go-v2/service/sts v1.34.1
	github.com/golang/mock v1.6.0
	github.com/prometheus/client_golang v1.20.5
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/common v0.65.0
	github.com/stretchr/testify v1.10.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.71 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.33 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.37 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.37 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.4 // indirect
	github.com/aws/smithy-go v1.22.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
