package aws

import (
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

func TestGetRouteTableIDs(t *testing.T) {
	tests := []struct {
		name     string
		EC2API   *MockEC2API
		VpcID    string
		expected []string
	}{
		{
			name:     "Returns both a public and private route table id",
			EC2API:   &MockEC2API{},
			VpcID:    "test-vpc-id",
			expected: []string{"RouteTableId1", "RouteTableId2", "RouteTableId3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := GetRouteTableIDs(tc.EC2API, tc.VpcID)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf(`Expected result %v, Got %v`, tc.expected, result)
			}
		})
	}
}

func TestGetCustomerGatewayConfig(t *testing.T) {
	tests := []struct {
		name              string
		EC2API            *MockEC2API
		customerGatewayIP string
		stack             *cloudformation.Stack
		expected          string
	}{
		{
			name:              "Returns a VPN config for a specific CGW IP",
			EC2API:            &MockEC2API{},
			customerGatewayIP: "test-CustomerGatewayIP",
			stack: &cloudformation.Stack{
				Outputs: []*cloudformation.Output{
					&cloudformation.Output{
						Description: aws.String("test-Description"),
						ExportName:  aws.String("test-ExportName"),
						OutputKey:   aws.String("CustomerGateway0"),
						OutputValue: aws.String("test-customerGatewayID"),
					},
				},
			},
			expected: "this config contains test-CustomerGatewayIP",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := GetCustomerGatewayConfig(tc.EC2API, tc.customerGatewayIP, tc.stack)
			if result != tc.expected {
				t.Errorf(`Expected result %v, Got %v`, tc.expected, result)
			}
		})
	}
}

func TestGetVpcIDs(t *testing.T) {
	type args struct {
		ec2Svc ec2iface.EC2API
		nodes  []*string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "should retrun all unique vpcids",
			args: args{
				ec2Svc: &MockEC2API{
					VpcIds: map[string]string{
						"i-1234": "vpc1",
						"i-5678": "vpc2",
						"i-90ab": "vpc2",
					},
				},
			},
			want: []string{"vpc1", "vpc2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetVpcIDs(tt.args.ec2Svc, tt.args.nodes)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVpcIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			sort.Strings(got)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetVpcIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}
