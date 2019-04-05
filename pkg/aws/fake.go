package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

// MockCloudformationAPI provides mocked interface to AWS Cloudformation service
type MockCloudformationAPI struct {
	cloudformationiface.CloudFormationAPI

	Err    error
	Status string

	FailDescribe bool
}

//DescribeStacks mocks the cloudformation DescribeStacks call and returns a sample VPN stack output
func (m *MockCloudformationAPI) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if m.FailDescribe {
		return nil, m.Err
	}

	return &cloudformation.DescribeStacksOutput{
		Stacks: []*cloudformation.Stack{
			&cloudformation.Stack{
				StackName:   aws.String("foo"),
				StackStatus: aws.String(m.Status),
				Outputs: []*cloudformation.Output{
					&cloudformation.Output{
						OutputKey:   aws.String("CustomerGateway0"),
						OutputValue: aws.String("test-CustomerGatewayID"),
					},
				},
			},
		},
	}, nil
}

//CreateStack mocks the cloudformation CreateStack call and returns an empty output
func (m *MockCloudformationAPI) CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return &cloudformation.CreateStackOutput{}, nil
}

//DeleteStack mocks the cloudformation DeleteStack call and returns an empty output
func (m *MockCloudformationAPI) DeleteStack(*cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	return &cloudformation.DeleteStackOutput{}, nil
}

//MockEC2API mocks the ec2 api interface
type MockEC2API struct {
	ec2iface.EC2API
	VpcIds map[string]string
}

//DescribeRouteTables mocks the cloudformation DescribeRouteTables call and returns test output
func (m *MockEC2API) DescribeRouteTables(*ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{
		NextToken: aws.String(""),
		RouteTables: []*ec2.RouteTable{
			&ec2.RouteTable{Associations: []*ec2.RouteTableAssociation{
				&ec2.RouteTableAssociation{
					Main:                    aws.Bool(true),
					RouteTableAssociationId: aws.String("test-RouteTableAssociationId"),
					RouteTableId:            aws.String("test-RouteTableId"),
					SubnetId:                aws.String("test-SubnetId"),
				},
			},
				OwnerId:         aws.String("test-OwnerId"),
				PropagatingVgws: []*ec2.PropagatingVgw{&ec2.PropagatingVgw{}},
				RouteTableId:    aws.String("PublicRouteTableId"),
				Routes: []*ec2.Route{
					&ec2.Route{},
				},
				Tags: []*ec2.Tag{&ec2.Tag{
					Key:   aws.String("foo"),
					Value: aws.String("PublicRouteTable"),
				}},
				VpcId: aws.String("test-vpc-id"),
			},
			&ec2.RouteTable{Associations: []*ec2.RouteTableAssociation{
				&ec2.RouteTableAssociation{
					Main:                    aws.Bool(true),
					RouteTableAssociationId: aws.String("test-RouteTableAssociationId"),
					RouteTableId:            aws.String("test-RouteTableId"),
					SubnetId:                aws.String("test-SubnetId"),
				},
			},
				OwnerId:         aws.String("test-OwnerId"),
				PropagatingVgws: []*ec2.PropagatingVgw{&ec2.PropagatingVgw{}},
				RouteTableId:    aws.String("PrivateRouteTableId"),
				Routes: []*ec2.Route{
					&ec2.Route{},
				},
				Tags: []*ec2.Tag{&ec2.Tag{
					Key:   aws.String("foo"),
					Value: aws.String("PrivateRouteTable"),
				}},
				VpcId: aws.String("test-vpc-id"),
			},
		},
	}, nil
}

//DescribeVpnConnections mocks the cloudformation DescribeVpnConnections call and returns test output
func (m *MockEC2API) DescribeVpnConnections(*ec2.DescribeVpnConnectionsInput) (*ec2.DescribeVpnConnectionsOutput, error) {
	return &ec2.DescribeVpnConnectionsOutput{
		VpnConnections: []*ec2.VpnConnection{
			&ec2.VpnConnection{
				Category:                     aws.String("test-Category"),
				CustomerGatewayConfiguration: aws.String("this config contains test-CustomerGatewayIP"),
				CustomerGatewayId:            aws.String("test-customerGatewayID"),
				Options:                      &ec2.VpnConnectionOptions{},
				Routes: []*ec2.VpnStaticRoute{
					&ec2.VpnStaticRoute{},
				},
				State:            aws.String("test-State"),
				Tags:             []*ec2.Tag{&ec2.Tag{}},
				TransitGatewayId: aws.String("test-TransitGatewayId"),
				Type:             aws.String("test-Type"),
				VpnConnectionId:  aws.String("test-VpnConnectionId"),
				VpnGatewayId:     aws.String("test-VpnGatewayId"),
			},
		},
	}, nil
}

//DescribeInstances mocks the ec2 DescribeInstances call and returns the instances stored in the Mock
func (m *MockEC2API) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	out := &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{},
	}
	for instanceId, vpcID := range m.VpcIds {
		out.Reservations = append(out.Reservations, &ec2.Reservation{
			Instances: []*ec2.Instance{&ec2.Instance{
				InstanceId: aws.String(instanceId),
				VpcId:      aws.String(vpcID),
			}},
		})
	}
	return out, nil
}
