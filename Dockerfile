# Build the manager binary
FROM golang:1.10.3 as builder
LABEL maintainer="Chris Krough <ckkrough@amazon.com>"

# Copy in the go src
WORKDIR /go/src/github.com/awslabs/aws-vpn-controller
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/awslabs/aws-vpn-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM ubuntu:latest
WORKDIR /
COPY --from=builder /go/src/github.com/awslabs/aws-vpn-controller/manager .
COPY --from=builder /etc/ssl /etc/ssl
ENTRYPOINT ["/manager"]
