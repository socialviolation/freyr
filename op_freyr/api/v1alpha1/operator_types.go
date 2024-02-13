/*
Copyright 2024 nick@fmtl.au.

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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OperatorSpec defines the desired state of Operator
type OperatorSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=weather;trig
	// +kubebuilder:validation:default:=weather
	Mode string `json:"mode,omitempty"`
	// +kubebuilder:validation:Optional
	Weather WeatherMode `json:"weather,omitempty"`
	// +kubebuilder:validation:Optional
	Trig TrigMode `json:"trig,omitempty"`
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
	Period int32 `json:"period,omitempty"`
	Min    int32 `json:"min,omitempty"`
	Max    int32 `json:"max,omitempty"`
}

// OperatorStatus defines the observed state of Operator
type OperatorStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Operator is the Schema for the operators API
type Operator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperatorSpec   `json:"spec,omitempty"`
	Status OperatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OperatorList contains a list of Operator
type OperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Operator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Operator{}, &OperatorList{})
}
