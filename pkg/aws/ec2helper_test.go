package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

func TestGetRouteTableIDs(t *testing.T) {
	tests := []struct {
		name     string
		EC2API   *MockEC2API
		VpcID    string
		expected RouteTableIDs
	}{
		{
			name:   "Returns both a public and private route table id",
			EC2API: &MockEC2API{},
			VpcID:  "test-vpc-id",
			expected: RouteTableIDs{
				Public:  "PublicRouteTableId",
				Private: "PrivateRouteTableId",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := GetRouteTableIDs(tc.EC2API, tc.VpcID)
			if result != tc.expected {
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
