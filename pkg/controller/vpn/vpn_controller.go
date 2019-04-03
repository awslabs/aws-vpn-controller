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
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	networkingv1alpha1 "github.com/awslabs/aws-vpn-controller/pkg/apis/networking/v1alpha1"
	awsHelper "github.com/awslabs/aws-vpn-controller/pkg/aws"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("vpn-controller")

// Add creates a new VPN Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		metadatasvc := ec2metadata.New(session.Must(session.NewSession()))
		d, err := metadatasvc.GetInstanceIdentityDocument()
		if err != nil {
			log.Error(err, "error setting region")
			panic(err)
		}
		region = d.Region
	}
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(region)}))

	return &ReconcileVPN{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		sess:   sess,
		cfnSvc: cloudformation.New(sess),
		ec2Svc: ec2.New(sess),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("vpn-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to VPN
	err = c.Watch(&source.Kind{Type: &networkingv1alpha1.VPN{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileVPN{}

// ReconcileVPN reconciles a VPN object
type ReconcileVPN struct {
	client.Client
	scheme *runtime.Scheme
	sess   *session.Session
	cfnSvc cloudformationiface.CloudFormationAPI
	ec2Svc ec2iface.EC2API
}

type cfnTemplateInput struct {
	VpcID               string
	VPNConnections      []networkingv1alpha1.VPNConnection
	PublicRouteTableID  string
	PrivateRouteTableID string
}

// Status Codes for the VPN Object
var (
	StatusCreateComplete = "Complete"
	StatusCreating       = "Creating"
	StatusFailed         = "Failed"
)

func getStackName(instance *networkingv1alpha1.VPN) string {
	return fmt.Sprintf("awsvpnctl-%s-%s", instance.Namespace, instance.GetName())
}

func containsString(s []string, t string) bool {
	for _, a := range s {
		if a == t {
			return true
		}
	}
	return false
}

func removeString(s []string, t string) []string {
	for i, e := range s {
		if e == t {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// Reconcile reads that state of the cluster for a VPN object and makes changes based on the state read
// and what is in the VPN.Spec
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.amazonaws.com,resources=vpns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.amazonaws.com,resources=vpns/status,verbs=get;update;patch
func (r *ReconcileVPN) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := log.WithValues("name", request.Name)

	instance := &networkingv1alpha1.VPN{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	stackName := getStackName(instance)
	finalizer := "vpn.networking.amazonaws.com"
	stack, err := awsHelper.DescribeStack(r.cfnSvc, stackName)

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(instance.ObjectMeta.Finalizers, finalizer) {
			log.Info("adding finalizer", "instance", instance.GetName(), "finalizer", finalizer)
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizer)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		if containsString(instance.ObjectMeta.Finalizers, finalizer) {
			if err != nil && awsHelper.StackDoesNotExist(err) {
				log.Info("removing finalizer", "instance", instance.GetName(), "finalizer", finalizer)
				instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, finalizer)
				return reconcile.Result{}, r.Update(context.TODO(), instance)
			}

			if err != nil {
				log.Error(err, "error describing stack", "stackName", stackName, "instance", instance.GetName())
				return reconcile.Result{}, err
			}

			if *stack.StackStatus == cloudformation.StackStatusDeleteComplete {
				log.Info("stack deleted", "stackName", stackName, "instance", instance.GetName(), "finalizer", finalizer)
				instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, finalizer)
				return reconcile.Result{}, r.Update(context.TODO(), instance)
			}

			if *stack.StackStatus == cloudformation.StackStatusDeleteInProgress {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}

			log.Info("deleting stack", "stackName", stackName, "instance", instance.GetName())
			_, err = r.cfnSvc.DeleteStack(&cloudformation.DeleteStackInput{StackName: aws.String(stackName)})
			if err != nil {
				log.Error(err, "stackName", stackName, "instance", instance.GetName())
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
	}

	if err != nil && awsHelper.StackDoesNotExist(err) {
		log.Info("creating stack", "stackName", stackName, "instance", instance.GetName())

		if err = r.createVPNStack(instance); err != nil {
			instance.Status.Status = StatusFailed
			r.Update(context.TODO(), instance)
			return reconcile.Result{}, err
		}
		instance.Status.Status = StatusCreating
		r.Update(context.TODO(), instance)
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "error creating stack", "stackName", stackName, "instance", instance.GetName())
		return reconcile.Result{}, err
	}

	if awsHelper.IsFailed(*stack.StackStatus) {
		log.Info("stack in failed state", "stackStatus", *stack.StackStatus, "stackName", stackName, "instance", instance.GetName())
		instance.Status.Status = StatusFailed
		err = r.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}

	if awsHelper.IsComplete(*stack.StackStatus) {

		for _, c := range instance.Spec.VPNConnections {
			customerGatewayConfig, err := awsHelper.GetCustomerGatewayConfig(r.ec2Svc, c.CustomerGatewayIP, stack)
			if err != nil {
				log.Error(err, "unable to get customer gateway config", "stackName", stackName, "instance", instance.GetName())
				return reconcile.Result{}, err
			}

			if err = r.updateVPNConfigToSecret(c.ConfigSecretName, instance.Namespace, customerGatewayConfig); err != nil {
				return reconcile.Result{}, err
			}
		}

		instance.Status.Status = StatusCreateComplete
		log.Info("stack complete", "stackName", stackName, "stackStatus", *stack.StackStatus, "instance", instance.GetName())
		return reconcile.Result{}, r.Update(context.TODO(), instance)

	}

	if awsHelper.IsPending(*stack.StackStatus) {
		log.Info("waiting for stack to complete", "stackName", stackName, "stackStatus", *stack.StackStatus, "instance", instance.GetName())
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	log.Info("stack in unknown state", "stackName", stackName, "stackStatus", *stack.StackStatus, "instance", instance.GetName())
	instance.Status.Status = StatusFailed
	err = r.Update(context.TODO(), instance)
	return reconcile.Result{}, err

}

func (r *ReconcileVPN) updateVPNConfigToSecret(secretname string, namespace string, vpnConfig string) error {
	secret := &corev1.Secret{}
	if err := r.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: secretname}, secret); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Unable to find the secret %s/%s", namespace, secret)
		}
		return err
	}

	data := map[string][]byte{
		"VPNConfiguration": []byte(vpnConfig),
	}
	if !reflect.DeepEqual(secret.Data, data) {
		log.Info("vpn config changed, updating secret", "secretName", secretname)
		secret.Data = data
		if err := r.Update(context.TODO(), secret); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileVPN) createVPNStack(instance *networkingv1alpha1.VPN) error {
	vpcID := instance.Spec.VpcID
	var err error
	if vpcID == "" {
		vpcID, err = getVpcID(r, r.ec2Svc)
		if err != nil {
			log.Error(err, "could not determin the vpcID")
			return err
		}
	}

	rtbs, err := awsHelper.GetRouteTableIDs(r.ec2Svc, vpcID)
	if err != nil {
		return err
	}

	cfnTemplate, err := awsHelper.GetCFNTemplateBody(vpnCFNTemplate, cfnTemplateInput{
		VpcID:               vpcID,
		VPNConnections:      instance.Spec.VPNConnections,
		PrivateRouteTableID: rtbs.Private,
		PublicRouteTableID:  rtbs.Public,
	})
	if err != nil {
		return err
	}

	_, err = r.cfnSvc.CreateStack(&cloudformation.CreateStackInput{
		TemplateBody: aws.String(cfnTemplate),
		StackName:    aws.String(getStackName(instance)),
		Capabilities: []*string{aws.String("CAPABILITY_NAMED_IAM"), aws.String("CAPABILITY_IAM")},
		Tags:         []*cloudformation.Tag{},
	})

	return err
}

// getVpcID takes the nodes of the cluster, and returns the vpcID of them if they all match.
func getVpcID(nodeLister client.Client, ec2Svc ec2iface.EC2API) (string, error) {
	// TODO get a list of nodes
	nodes := &corev1.NodeList{}
	err := nodeLister.List(context.TODO(), &client.ListOptions{}, nodes)
	if err != nil {
		log.Error(err, "error getting nodes")
		return "", err
	}

	instanceIds := []*string{}
	for _, node := range nodes.Items {
		parts := strings.Split(node.Spec.ProviderID, "/")
		if len(parts) < 5 {
			err := fmt.Errorf("node provider spec is not valid")
			log.WithValues("instanceID", node.Spec.ProviderID).Error(err, "could not parse ProviderID")
			return "", err
		}
		instanceIds = append(instanceIds, aws.String(parts[4]))
	}

	ids, err := awsHelper.GetVpcIDs(ec2Svc, instanceIds)
	if err != nil {
		log.Error(err, "describing ec2 instances")
		return "", err
	}
	if len(ids) > 1 {
		err := fmt.Errorf("multiple vpcid found")
		log.WithValues("vpcIds", ids).Error(err, "more then one vpcid found not guessing")
		return "", err
	}

	return ids[0], nil
}
