package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

// GetVpcIDs retrieves a list of unique vpcIDs from the provided nodes.
func GetVpcIDs(ec2Svc ec2iface.EC2API, nodes []*string) ([]string, error) {
	resp, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: nodes,
	})
	if err != nil {
		return nil, err
	}
	ids := map[string]struct{}{}
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			ids[*instance.VpcId] = struct{}{}
		}
	}
	ret := make([]string, 0, len(ids))
	for k := range ids {
		ret = append(ret, k)
	}

	return ret, nil
}

//GetRouteTableIDs takes vpc ID and returns its routetable ids
func GetRouteTableIDs(ec2Svc ec2iface.EC2API, VpcID string) ([]string, error) {
	ids := []string{}
	filter := []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: []*string{aws.String(VpcID)},
		},
	}
	routeTables, err := ec2Svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{Filters: filter})
	if err != nil {
		return ids, err
	}

	for _, r := range routeTables.RouteTables {
		ids = append(ids, aws.StringValue(r.RouteTableId))
	}

	if len(ids) > 0 {
		return ids, nil
	}

	err = fmt.Errorf("route table ids not found")
	return ids, err

}

//GetCustomerGatewayConfig takes a customer gateway IP address and a CFN VPN stack name and returns the VPN config for that customer gateway
func GetCustomerGatewayConfig(ec2Svc ec2iface.EC2API, customerGatewayIP string, stack *cloudformation.Stack) (string, error) {

	for _, output := range stack.Outputs {
		if strings.HasPrefix(*output.OutputKey, "CustomerGateway") {
			customerGatewayID := *output.OutputValue

			vpnFilter := []*ec2.Filter{
				{
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
