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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	VPNSpec             *networkingv1alpha1.VPNSpec
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
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.amazonaws.com,resources=vpns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.amazonaws.com,resources=vpns/status,verbs=get;update;patch
func (r *ReconcileVPN) Reconcile(request reconcile.Request) (reconcile.Result, error) {

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

			log.Info("deleting secrets", "instance", instance.GetName())
			for _, vpn := range instance.Spec.VPNConnections {
				if err = r.deleteSecret(vpn.ConfigSecretName, instance.Namespace); err != nil {
					return reconcile.Result{}, err
				}
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

			if err = r.storeVPNConfigToSecret(c.ConfigSecretName, instance.Namespace, customerGatewayConfig); err != nil {
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

func (r *ReconcileVPN) createSecret(secretname string, namespace string, vpnConfig string) error {
	log.Info("creating secret", "secretName", secretname)
	if err := r.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretname,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"VPNConfiguration": []byte(vpnConfig),
		},
	}); err != nil {
		log.Error(err, "error creating secret", "secretName", secretname)
		return err
	}

	return nil
}

func (r *ReconcileVPN) deleteSecret(secretname string, namespace string) error {
	log.Info("deleting secret", "secretName", secretname)
	found := &corev1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: secretname}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("secret not found", "secretName", secretname)
			return nil
		}
		log.Error(err, "error getting secret", "secretName", secretname)
		return err
	}

	if err = r.Delete(context.TODO(), found); err != nil {
		log.Error(err, "error deleting secret", "secretName", secretname)
		return err
	}

	return nil
}

func (r *ReconcileVPN) storeVPNConfigToSecret(secretname string, namespace string, vpnConfig string) error {
	secret := &corev1.Secret{}
	if err := r.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: secretname}, secret); err != nil {
		if errors.IsNotFound(err) {
			return r.createSecret(secretname, namespace, vpnConfig)
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
	rtbs, err := awsHelper.GetRouteTableIDs(r.ec2Svc, instance.Spec.VpcID)
	if err != nil {
		return err
	}

	cfnTemplate, err := awsHelper.GetCFNTemplateBody(vpnCFNTemplate, cfnTemplateInput{
		VPNSpec:             &instance.Spec,
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
