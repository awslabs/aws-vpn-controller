package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

// RouteTableIDs contains IDs for the public and private route tables for a VPC
type RouteTableIDs struct {
	Public  string
	Private string
}

//GetRouteTableIDs takes a vpn instance and returns its public and private routetable ids
func GetRouteTableIDs(ec2Svc ec2iface.EC2API, VpcID string) (RouteTableIDs, error) {
	ids := RouteTableIDs{}
	filter := []*ec2.Filter{
		&ec2.Filter{
			Name:   aws.String("vpc-id"),
			Values: []*string{aws.String(VpcID)},
		},
	}
	routeTables, err := ec2Svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{Filters: filter})
	if err != nil {
		return RouteTableIDs{}, err
	}

	for _, r := range routeTables.RouteTables {
		for _, v := range r.Tags {
			if aws.StringValue(v.Value) == "PublicRouteTable" {
				ids.Public = aws.StringValue(r.RouteTableId)
			}
			if aws.StringValue(v.Value) == "PrivateRouteTable" {
				ids.Private = aws.StringValue(r.RouteTableId)
			}
		}
	}

	if ids.Private != "" && ids.Public != "" {
		return ids, nil
	}

	err = fmt.Errorf("route table ids not found")
	return RouteTableIDs{}, err

}

//GetCustomerGatewayConfig takes a customer gateway IP address and a CFN VPN stack name and returns the VPN config for that customer gateway
func GetCustomerGatewayConfig(ec2Svc ec2iface.EC2API, customerGatewayIP string, stack *cloudformation.Stack) (string, error) {

	for _, output := range stack.Outputs {
		if strings.HasPrefix(*output.OutputKey, "CustomerGateway") {
			customerGatewayID := *output.OutputValue

			vpnFilter := []*ec2.Filter{
				&ec2.Filter{
					Name:   aws.String("customer-gateway-id"),
					Values: []*string{aws.String(customerGatewayID)},
				},
			}

			vpnConnection, err := ec2Svc.DescribeVpnConnections(&ec2.DescribeVpnConnectionsInput{Filters: vpnFilter})
			if err != nil {
				return "", err
			}

			if strings.Contains(*vpnConnection.VpnConnections[0].CustomerGatewayConfiguration, customerGatewayIP) {
				return *vpnConnection.VpnConnections[0].CustomerGatewayConfiguration, nil
			}

		}
	}

	return "", fmt.Errorf("unable to get CustomerGatewayID from cloudformation stack")
}
