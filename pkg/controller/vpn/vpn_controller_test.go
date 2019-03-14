/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vpn

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	networkingv1alpha1 "github.com/awslabs/aws-vpn-controller/pkg/apis/networking/v1alpha1"
	awsHelper "github.com/awslabs/aws-vpn-controller/pkg/aws"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MockCloudformationAPI struct {
	cloudformationiface.CloudFormationAPI

	Err    error
	Status string

	FailCreate   bool
	FailDescribe bool
	FailDelete   bool
}

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var vpnKey = types.NamespacedName{Name: "foo", Namespace: "default"}

const timeout = time.Second * 5

func newTestReconciler(mgr manager.Manager) *ReconcileVPN {
	var errDoesNotExist = awserr.New("ValidationError", "Stack with id awsvpnctl-foo-testvpn does not exist", nil)
	return &ReconcileVPN{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		cfnSvc: MockCloudformationAPI{FailDescribe: true, Err: errDoesNotExist},
		ec2Svc: &awsHelper.MockEC2API{},
	}
}
func TestGetStackName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &networkingv1alpha1.VPN{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
	name := getStackName(instance)
	g.Expect(name).To(gomega.Equal("awsvpnctl-default-foo"))
}
func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &networkingv1alpha1.VPN{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: networkingv1alpha1.VPNSpec{
			VpcID: "test-VpcID",
			VPNConnections: []networkingv1alpha1.VPNConnection{
				networkingv1alpha1.VPNConnection{
					CustomerGatewayIP: "test-CustomerGatewayIP",
					ConfigSecretName:  "test-configsecretname",
				},
			},
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	errDoesNotExist := awserr.New("ValidationError", `ValidationError: Stack with id awsvpnctl-default-foo does not exist, status code: 400, request id: 42`, nil)
	reconcile := &ReconcileVPN{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		cfnSvc: &awsHelper.MockCloudformationAPI{FailDescribe: true, Err: errDoesNotExist, Status: cloudformation.StackStatusCreateComplete},
		ec2Svc: &awsHelper.MockEC2API{},
	}

	recFn, requests := SetupTestReconcile(reconcile)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configsecretname",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}
	err = c.Create(context.TODO(), secret)
	if err != nil {
		t.Logf("failed to create test secret: %v", err)
		return
	}

	// Create the VPN object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getVPN := &networkingv1alpha1.VPN{}
	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), vpnKey, getVPN)
		return getVPN.Status.Status, err
	}).Should(gomega.Equal(StatusCreating))

	reconcile.cfnSvc = &awsHelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateComplete}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}
