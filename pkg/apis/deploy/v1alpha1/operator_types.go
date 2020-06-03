// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GenericContainerSpec struct {
	Enabled *bool    `json:"enabled,omitempty"`
	Name    *string  `json:"name,omitempty"`
	Image   *string  `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

var (
	ContainerEnvVarKeyWATCHNAMESPACE = "WATCH_NAMESPACE"
	ContainerEnvVarKeyPODNAME        = "POD_NAME"
	ContainerEnvVarKeyOPERATORNAME   = "OPERATOR_NAME"

	DefaultPodServiceAccountName = "ham-deploy"
)

var (
	DefaultDeployableEnablement       = true
	DefaultDeployableContainerName    = "deployable"
	DefaultDeployableContainerImage   = "quay.io/hybridappio/ham-deployable-operator"
	DefaultDeployableContainerCommand = []string{"ham-deployable-operator"}
)

type DeployableOperatorSpec struct {
	GenericContainerSpec `json:",inline"`
}

var (
	DefaultAssmeblerEnablement       = true
	DefaultAssemblerContainerName    = "assembler"
	DefaultAssemblerContainerImage   = "quay.io/hybridappio/ham-application-assembler"
	DefaultAssemblerContainerCommand = []string{"ham-application-assembler"}
)

type ApplicationAssemblerSpec struct {
	GenericContainerSpec `json:",inline"`
}

var (
	DefaultDiscovererEnablement       = true
	DefaultDiscovererContainerName    = "discoverer"
	DefaultDiscovererContainerImage   = "quay.io/hybridappio/ham-resource-discoverer"
	DefaultDiscovererContainerCommand = []string{"ham-resource-discoverer"}

	DefaultPodVolumeNameHubConnection = "hub-connection-config"
)

var (
	ContainerEnvVarKeyCLUSTERNAME      = "CLUSTERNAME"
	ContainerEnvVarKeyCLUSTERNAMESPACE = "CLUSTERNAMESPACE"
	ContainerEnvVarKeyHUBKUBECONFIG    = "HUBCLUSTERCONFIGFILE"
)

type HubConnectionConfig struct {
	KubeConfig *string                     `json:"kubeconfig,omitempty"`
	MountPath  string                      `json:"mountpath"`
	SecretRef  corev1.LocalObjectReference `json:"secretRef"`
}

type ResourceDiscovererSpec struct {
	GenericContainerSpec `json:",inline"`
	ClusterName          string               `json:"clustername"`
	ClusterNamespace     string               `json:"clusternamespace"`
	HubConnectionConfig  *HubConnectionConfig `json:"hubconfig,omitempty"`
}

type CoreSpec struct {
	DeployableOperatorSpec *DeployableOperatorSpec `json:"deployable,omitempty"`
}

type ToolsSpec struct {
	ApplicationAssemblerSpec *ApplicationAssemblerSpec `json:"assembler,omitempty"`
	ResourceDiscovererSpec   *ResourceDiscovererSpec   `json:"discoverer,omitempty"`
}

type LicenseSpec struct {
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Accept terms and conditions"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	Accept bool `json:"accept"`
}

// OperatorSpec defines the desired state of Operator
type OperatorSpec struct {
	LicenseSpec *LicenseSpec `json:"license"`
	CoreSpec    *CoreSpec    `json:"core,omitempty"`
	ToolsSpec   *ToolsSpec   `json:"tools,omitempty"`
}

type Phase string

const (
	PhaseInstalled Phase = "Installed"
	PhasePending   Phase = "Pending"
	PhaseError     Phase = "Error"
)

// OperatorStatus defines the observed state of Operator
type OperatorStatus struct {
	// +kubebuilder:validation:Enum=Installed;Pending;Error
	Phase     Phase             `json:"phase,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Message   string            `json:"message,omitempty"`
	PodStatus *corev1.PodStatus `json:"podstatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Operator is the Schema for the operators API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operators,scope=Namespaced
type Operator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperatorSpec   `json:"spec,omitempty"`
	Status OperatorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatorList contains a list of Operator
type OperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Operator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Operator{}, &OperatorList{})
}
