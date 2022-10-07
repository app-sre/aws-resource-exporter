package pkg

import (
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-kit/kit/log"
	"github.com/golang/mock/gomock"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

var timeout time.Duration = 10 * time.Second

func Test_collectIPv4BlocksPerVpcUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock := mock.NewMockClient(ctrl)
	vpc := ec2.Vpc{
		VpcId: aws.String("1"),
		CidrBlockAssociationSet: []*ec2.VpcCidrBlockAssociation{
			{},
		},
	}

	mock.EXPECT().DescribeVpcsWithContext(gomock.Any(), &ec2.DescribeVpcsInput{
		VpcIds: []*string{vpc.VpcId},
	}).Return(&ec2.DescribeVpcsOutput{
		Vpcs: []*ec2.Vpc{&vpc},
	}, nil)

	e := NewVPCExporter([]*session.Session{}, log.NewNopLogger(), VPCConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Interval: &timeout,
			Timeout:  &timeout,
			CacheTTL: &timeout,
		},
		Regions: []string{"any"},
	}, "1")
	e.collectIPv4BlocksPerVpcUsage(&vpc, mock, "any")
	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var dto dto.Metric
	metrics[0].Write(&dto)
	assert.Equal(t, float64(1), *dto.Gauge.Value)
}

func Test_collectRoutesPerTablePerVpcUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock := mock.NewMockClient(ctrl)
	rtb := ec2.RouteTable{
		VpcId:        aws.String("1"),
		RouteTableId: aws.String("1"),
		Routes: []*ec2.Route{
			{},
		},
	}

	mock.EXPECT().DescribeRouteTablesWithContext(gomock.Any(), &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{rtb.RouteTableId},
	}).Return(&ec2.DescribeRouteTablesOutput{
		RouteTables: []*ec2.RouteTable{&rtb},
	}, nil)

	e := NewVPCExporter([]*session.Session{}, log.NewNopLogger(), VPCConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Interval: &timeout,
			Timeout:  &timeout,
			CacheTTL: &timeout,
		},
		Regions: []string{"any"},
	}, "1")
	e.collectRoutesPerRouteTableUsage(&rtb, mock, "any")
	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var dto dto.Metric
	metrics[0].Write(&dto)
	assert.Equal(t, float64(1), *dto.Gauge.Value)
}
