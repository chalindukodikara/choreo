/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeploymentSpec defines the desired state of Deployment.
type DeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Number of deployment revisions to keep for rollback.
	// +optional (default: 10)
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Reference to the deployable artifact that is being deployed.
	// +required
	DeploymentArtifactRef string `json:"deploymentArtifactRef"`

	// Environment-specific configuration overrides applied to the artifact
	// before being deployed.
	// +optional
	ConfigurationOverrides *ConfigurationOverrides `json:"configurationOverrides,omitempty"`
}

// ConfigurationOverrides holds environment-specific overrides to the artifact configuration.
type ConfigurationOverrides struct {
	// Endpoint configuration overrides for this deployment.
	// +optional
	EndpointTemplates []EndpointOverride `json:"endpointTemplates,omitempty"`

	// Dependency configuration overrides for this deployment.
	// +optional
	Dependencies *DependenciesOverride `json:"dependencies,omitempty"`

	// Application configuration overrides for this deployment.
	// +optional
	Application *Application `json:"application,omitempty"`
}

// EndpointOverride captures overrides for an existing endpoint’s configuration.
type EndpointOverride struct {
	// TODO: Define the structure of the endpoint override.
}

// DependenciesOverride captures overrides for dependencies.
type DependenciesOverride struct {
	// TODO: Define the structure of the dependencies override.
}

// DeploymentStatus defines the observed state of Deployment.
type DeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Deployment is the Schema for the deployments API.
type Deployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentSpec   `json:"spec,omitempty"`
	Status DeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeploymentList contains a list of Deployment.
type DeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Deployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Deployment{}, &DeploymentList{})
}
