package vpn

var vpnCFNTemplate = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: VPN managed by the Kubernetes aws-vpn-controller
Resources:

  VPNGateway:
    Type: AWS::EC2::VPNGateway
    Properties:
      Type: ipsec.1

  VPCGatewayAttachment:
    Type: AWS::EC2::VPCGatewayAttachment
    Properties:
      VpcId: {{ .VpcID }}
      VpnGatewayId: !Ref 'VPNGateway'

  VPNGatewayRoutePropagation:
    Type: AWS::EC2::VPNGatewayRoutePropagation
    Properties:
      RouteTableIds:
{{- range $i, $e := .RouteTableIDs }}
        - {{ $e }}
{{- end }}
      VpnGatewayId: !Ref 'VPNGateway'
    DependsOn: VPCGatewayAttachment

{{- range $i, $e := .VPNConnections }}

  CustomerGateway{{ $i }}:
    Type: AWS::EC2::CustomerGateway
    Properties:
      Type: ipsec.1
      BgpAsn: 65000
      IpAddress: {{ $e.CustomerGatewayIP }}

  VPNConnection{{ $i }}:
    Type: AWS::EC2::VPNConnection
    Properties:
      Type: ipsec.1
      StaticRoutesOnly: false
      CustomerGatewayId: !Ref 'CustomerGateway{{ $i }}'
      VpnGatewayId: !Ref 'VPNGateway'
{{- end }} 

Outputs:
{{- range $i, $e := .VPNConnections }}
  
  CustomerGateway{{ $i }}:
      Description: Customer Gateway {{ $i }}
      Value: !Ref CustomerGateway{{ $i }}
{{- end }} 
`
