module github.com/app-sre/aws-resource-exporter

go 1.26.0

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/aws/aws-sdk-go-v2 v1.47.5
	github.com/aws/aws-sdk-go-v2/config v1.33.12
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.342.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.64.2
	github.com/aws/aws-sdk-go-v2/service/iam v1.64.6
	github.com/aws/aws-sdk-go-v2/service/kafka v1.65.7
	github.com/aws/aws-sdk-go-v2/service/rds v1.131.2
	github.com/aws/aws-sdk-go-v2/service/route53 v1.70.6
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.43.7
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.6
	github.com/aws/smithy-go v1.28.7
	github.com/golang/mock v1.6.0
	github.com/prometheus/client_golang v1.25.0
	github.com/prometheus/client_model v0.6.3
	github.com/prometheus/common v0.72.1
	github.com/stretchr/testify v1.12.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.2.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.49.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
