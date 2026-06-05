module github.com/app-sre/aws-resource-exporter

go 1.25.0

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/aws/aws-sdk-go-v2 v1.41.11
	github.com/aws/aws-sdk-go-v2/config v1.32.22
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.305.1
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.54.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.54.2
	github.com/aws/aws-sdk-go-v2/service/kafka v1.52.4
	github.com/aws/aws-sdk-go-v2/service/rds v1.119.0
	github.com/aws/aws-sdk-go-v2/service/route53 v1.63.1
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.35.4
	github.com/aws/aws-sdk-go-v2/service/sts v1.43.1
	github.com/aws/smithy-go v1.27.1
	github.com/golang/mock v1.6.0
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/common v0.68.1
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.28 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.1.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.31.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
