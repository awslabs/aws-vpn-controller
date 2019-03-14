# AWS VPN Controller

[![Build Status](https://travis-ci.org/awslabs/aws-vpn-controller.svg?branch=master)](https://travis-ci.org/awslabs/aws-vpn-controller)

## Overview

The AWS VPN Controller allows you to create and delete AWS VPNs and connect them to your VPCs using Kubernetes Custom Resource Definitions.

The `aws-vpn-controller` runs as a pod in your Kubernetes cluster and listens for new `VPN` type CRDs. When a new VPN Resource is created, the controller will help establish an [AWS Site-to-Site VPN](https://docs.aws.amazon.com/vpn/latest/s2svpn/VPC_VPN.html) by creating an [AWS VPNGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpnGateway.html) and one or more [VPN Connections](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpnConnection.html) and attaching them to the [VPC](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html) that you specify. The [Customer Gateway Configuration](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpnConnection.html) for each VPNConnection is stored in a k8s secret.

## Use Cases

The VPN controller can be used to establish VPN connections between your Kubernetes worker node's VPC and remote networks. The VPNs act like most other Resources in Kubernetes, and may be `kubectl apply`'d, etc.

Some example use cases:

- Securing traffic between the cluster and clients that are limited to unencrypted protocols
- Provide routability to privately addressed clients.

## Defining Types

The custom resource specification defines:

1. The ID (`vpcid`) of the AWS VPC that you would like the VPN Connections connected to
1. The Internet-routable IP address (`customergatewayip`) of the [customer gateway's](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CustomerGateway.html) outside interface that you would like the VPNConnection to terminate on.
1. The name of a pre-existing Kubernetes Secret (`configsecretname`) where the corresponding Customer Gateway Configuration should be stored.

When the VPN Connections are created by this resource, AWS will provide a VPN configuration that includes endpoint IP addresses, passphrases, and VPN details. On each Reconcile loop, that VPN configuration is pulled from the AWS API and stored in the `.data.VPNConfiguration` field of the secret specified by `configsecretname`. Your network automation tools can then pull that configuration from the secret for use in configuring network devices to terminate the AWS  Site-to-Site VPN. The VPN Resources are managed inside of a Cloudformation Stack with the naming pattern "awsvpnctl-Namespace-Instance", for example with a Kubernetes Namespace of `default` and a CRD VPN instance name of `samplevpn`, the resulting Cloudformation Stack name would be `awsvpnctl-default-samplevpn`.

### Example Spec

```yaml
apiVersion: networking.amazonaws.com/v1alpha1
kind: VPN
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: samplevpn
spec:
  vpcid: vpc-03f65a11d069da0d6
  vpnconnections:
    - customergatewayip: 8.8.8.8
      configsecretname: default-samplevpn-vpnconnection1-config
    - customergatewayip: 100.100.100.100
      configsecretname: default-samplevpn-vpnconnection2-config
```

This [controller](https://book.kubebuilder.io/basics/what_is_a_controller.html) is built on the [Kubebuilder](https://book.kubebuilder.io/) framework.

----

## Running Locally

Edit `config/samples/networking_v1alpha1_vpn.yaml` to include the `vpcid` and `vpnconnections` settings that you want to use.

1. build the controller, install the CRDs to your cluster, and run the code locally

    ```sh
    $ go get github.com/awslabs/aws-vpn-controller
    $ cd $GOPATH/src/github.com/awslabs/aws-vpn-controller
    $ make manifests install run
    go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
    CRD manifests generated under '/Users/ckkrough/go/src/github.com/awslabs/aws-vpn-controller/config/crds'
    RBAC manifests generated under '/Users/ckkrough/go/src/github.com/awslabs/aws-vpn-controller/config/rbac'
    kubectl apply -f config/crds
    customresourcedefinition.apiextensions.k8s.io "vpns.networking.amazonaws.com" configured
    go generate ./pkg/... ./cmd/...
    go fmt ./pkg/... ./cmd/...
    go vet ./pkg/... ./cmd/...
    go run ./cmd/manager/main.go
    {"level":"info","ts":1551460407.858978,"logger":"entrypoint","msg":"setting up client for manager"}
    {"level":"info","ts":1551460407.86035,"logger":"entrypoint","msg":"setting up manager"}
    {"level":"info","ts":1551460412.594866,"logger":"entrypoint","msg":"Registering Components."}
    {"level":"info","ts":1551460412.594893,"logger":"entrypoint","msg":"setting up scheme"}
    {"level":"info","ts":1551460412.595003,"logger":"entrypoint","msg":"Setting up controller"}
    {"level":"info","ts":1551460412.5950751,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"vpn-controller","source":"kind source: /, Kind="}
    {"level":"info","ts":1551460412.5951638,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"vpn-controller","source":"kind source: /, Kind="}
    {"level":"info","ts":1551460412.595204,"logger":"entrypoint","msg":"setting up webhooks"}
    {"level":"info","ts":1551460412.595211,"logger":"entrypoint","msg":"Starting the Cmd."}
    {"level":"info","ts":1551460412.899633,"logger":"kubebuilder.controller","msg":"Starting Controller","controller":"vpn-controller"}
    {"level":"info","ts":1551460413.000273,"logger":"kubebuilder.controller","msg":"Starting workers","controller":"vpn-controller","worker count":1}
    ```

1. create a custom resource of type VPN

    ```sh
    $ kubectl apply -f config/samples/networking_v1alpha1_vpn.yaml
    vpn.networking.amazonaws.com "samplevpn" created
    ```

1. check the vpns created through the custom resource

    ```sh
    $ kubectl get vpns
    NAME        AGE
    samplevpn   1m
    ```

1. retrieve the Customer Gateway Configuration

    ```sh
    $ k get secret default-samplevpn-vpnconnection1-config -o json | jq -r .data.VPNConfiguration | base64 --decode | xmllint --format -
    <?xml version="1.0" encoding="UTF-8"?>
    <vpn_connection id="vpn-0f72da61c0f80742a">
      <customer_gateway_id>cgw-0092e99221c25d208</customer_gateway_id>
      <vpn_gateway_id>vgw-095d6806b3d71eb6b</vpn_gateway_id>
      <vpn_connection_type>ipsec.1</vpn_connection_type>
      <ipsec_tunnel>
        ... snipped ...
      </ipsec_tunnel>
    </vpn_connection>
    ```

1. Delete the VPN:

    ```sh
    $ kubectl delete vpns samplevpn
    vpn.networking.amazonaws.com "samplevpn" deleted
    ```

## Installation

### Prerequisites

To get started you will need

- A Kubernetes cluster running in AWS. Check out [Amazon Elastic Container Service for Kubernetes](https://docs.aws.amazon.com/eks/latest/userguide/what-is-eks.html) or [Kubernetes Operations (kops)](https://github.com/kubernetes/kops) to get started.
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl) running on your workstation
- [awscli](https://docs.aws.amazon.com/cli/latest/userguide/installing.html) configured for your AWS account and region
- the controller uses the IAM Role of the k8s nodes for creating resources, so please ensure they have the appropriate permissions.

### Build

```sh
$ go get github.com/awslabs/aws-vpn-controller

$ cd $GOPATH/src/github.com/awslabs/aws-vpn-controller

# `IMG` is used to identify the container repo where the image will be stored
# `ROLEARN` is the ARN of the IAM role that the controller will assume when creating AWS Resources
# Example: "IMG="123456789123.dkr.ecr.us-west-2.amazonaws.com/aws-vpn-controller:latest" ROLEARN="arn:aws:iam::123456789123:role/aws-vpn-controller" make docker-build"
$ IMG="<your repo>/aws-vpn-controller:latest" ROLEARN="<IAM ROLE ARN>" make docker-build
go generate ./pkg/... ./cmd/...
go fmt ./pkg/... ./cmd/...
go vet ./pkg/... ./cmd/...
go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
CRD manifests generated under '/Users/ckkrough/go/src/github.com/awslabs/aws-vpn-controller/config/crds'
RBAC manifests generated under '/Users/ckkrough/go/src/github.com/awslabs/aws-vpn-controller/config/rbac'
go test ./pkg/... ./cmd/... -coverprofile cover.out
?       github.com/awslabs/aws-vpn-controller/pkg/apis  [no test files]
?       github.com/awslabs/aws-vpn-controller/pkg/apis/networking       [no test files]
ok      github.com/awslabs/aws-vpn-controller/pkg/apis/networking/v1alpha1      6.846s  coverage: 29.3% of statements
ok      github.com/awslabs/aws-vpn-controller/pkg/aws   0.147s  coverage: 56.7% of statements
?       github.com/awslabs/aws-vpn-controller/pkg/controller    [no test files]
ok      github.com/awslabs/aws-vpn-controller/pkg/controller/vpn        7.117s  coverage: 56.3% of statements
?       github.com/awslabs/aws-vpn-controller/pkg/webhook       [no test files]
?       github.com/awslabs/aws-vpn-controller/cmd/manager       [no test files]
docker build . -t aws-vpn-controller:latest
Sending build context to Docker daemon  316.2MB
Step 1/11 : FROM golang:1.10.3 as builder
 ---> d0e7a411e3da
Step 2/11 : LABEL maintainer="Chris Krough <ckkrough@amazon.com>"
 ---> Using cache
 ---> 0ff27ee6e65b
Step 3/11 : WORKDIR /go/src/github.com/awslabs/aws-vpn-controller
 ---> Using cache
 ---> 3a58b7582e7b
Step 4/11 : COPY pkg/    pkg/
 ---> Using cache
 ---> 09bf2725a96c
Step 5/11 : COPY cmd/    cmd/
 ---> Using cache
 ---> db095dbf88c6
Step 6/11 : COPY vendor/ vendor/
 ---> Using cache
 ---> 7757fa5ea873
Step 7/11 : RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/awslabs/aws-vpn-controller/cmd/manager
 ---> Using cache
 ---> ca7547bbd130
Step 8/11 : FROM ubuntu:latest
 ---> 1d9c17228a9e
Step 9/11 : WORKDIR /
 ---> Using cache
 ---> 00200ff44b12
Step 10/11 : COPY --from=builder /go/src/github.com/awslabs/aws-vpn-controller/manager .
 ---> Using cache
 ---> d52bbb64e83d
Step 11/11 : ENTRYPOINT ["/manager"]
 ---> Using cache
 ---> ca3031d04c3f
Successfully built ca3031d04c3f
Successfully tagged aws-vpn-controller:latest
updating kustomize image patch file for manager resource
sed -i'' -e 's@image: .*@image: '"aws-vpn-controller:latest"'@' ./config/default/manager_image_patch.yaml
```

### Push

This assumes your have a Docker repository configured.

```sh
$ $ IMG="<your repo>/aws-vpn-controller:latest" make docker-push
The push refers to repository [<your repo>/aws-vpn-controller]
2748ccaf68f7: Pushed
2c77720cf318: Pushed
1f6b6c7dc482: Pushed
c8dbbe73b68c: Pushed
2fb7bfc6145d: Pushed
latest: digest: sha256:3f324501c25a3ea790f18051480233a964766daeb347b2c00a5b51d134917bdc size: 1362
```

### Deploy

```sh
$ make deploy
go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
CRD manifests generated under '/Users/ckkrough/go/src/github.com/awslabs/aws-vpn-controller/config/crds'
RBAC manifests generated under '/Users/ckkrough/go/src/github.com/awslabs/aws-vpn-controller/config/rbac'
kubectl apply -f config/crds
customresourcedefinition.apiextensions.k8s.io "vpns.networking.amazonaws.com" configured
kustomize build config/default | kubectl apply -f -
2019/03/04 09:51:04 Adding nameprefix and namesuffix to Namespace resource will be deprecated in next release.
namespace "aws-vpn-controller-system" configured
clusterrole.rbac.authorization.k8s.io "aws-vpn-controller-manager-role" configured
clusterrole.rbac.authorization.k8s.io "aws-vpn-controller-proxy-role" configured
clusterrolebinding.rbac.authorization.k8s.io "aws-vpn-controller-manager-rolebinding" configured
clusterrolebinding.rbac.authorization.k8s.io "aws-vpn-controller-proxy-rolebinding" configured
secret "aws-vpn-controller-webhook-server-secret" unchanged
service "aws-vpn-controller-controller-manager-metrics-service" unchanged
service "aws-vpn-controller-controller-manager-service" unchanged
statefulset.apps "aws-vpn-controller-controller-manager" configured
```
