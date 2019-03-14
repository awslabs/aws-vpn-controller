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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VPNSpec defines the desired state of VPN
type VPNSpec struct {
	VpcID string `json:"vpcid"`
	// +kubebuilder:validation:MaxItems=2
	// +kubebuilder:validation:MinItems=1
	VPNConnections []VPNConnection `json:"vpnconnections"`
}

// VPNConnection contains the Customer Gateway IP and name of the secret where the VPNConfig is stored
type VPNConnection struct {
	CustomerGatewayIP string `json:"customergatewayip"`
	ConfigSecretName  string `json:"configsecretname"`
}

// VPNStatus defines the observed state of VPN
type VPNStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	Status string `json:"status"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VPN is the Schema for the vpns API
// +k8s:openapi-gen=true
type VPN struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPNSpec   `json:"spec,omitempty"`
	Status VPNStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VPNList contains a list of VPN
type VPNList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPN `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPN{}, &VPNList{})
}
