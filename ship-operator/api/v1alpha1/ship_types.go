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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ShipSpec defines the desired state of Ship
type ShipSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=weather;trig
	// +kubebuilder:validation:default:=weather
	Mode string `json:"mode,omitempty"`
	// +kubebuilder:validation:Optional
	Weather WeatherMode `json:"weather,omitempty"`
	// +kubebuilder:validation:Optional
	Trig TrigMode `json:"trig,omitempty"`
	// +kubebuilder:validation:Optional
	Captain PodSpec `json:"captain,omitempty"`
	// +kubebuilder:validation:Optional
	Conscript PodSpec `json:"conscript,omitempty"`

	// +kubebuilder:validation:Optional
	EnvVars map[string]string `json:"envs"`
}

type PodSpec struct {
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:Optional
	EnvVars map[string]string `json:"envs"`
}

type WeatherMode struct {
	// +kubebuilder:validation:Required
	Country string `json:"country,omitempty"`
	// +kubebuilder:validation:Required
	City string `json:"city,omitempty"`
	// +kubebuilder:validation:Required
	APIKey string `json:"apiKey,omitempty"`
}

type TrigMode struct {
	Duration string `json:"duration,omitempty"`
	Min      int32  `json:"min,omitempty"`
	Max      int32  `json:"max,omitempty"`
}

// ShipStatus defines the observed state of Ship
type ShipStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Ship is the Schema for the ships API
type Ship struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShipSpec   `json:"spec,omitempty"`
	Status ShipStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ShipList contains a list of Ship
type ShipList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ship `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ship{}, &ShipList{})
}
