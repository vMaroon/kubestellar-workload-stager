/*
Copyright 2024 The KubeStellar Authors.

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

	"github.com/kubestellar/kubestellar/api/control/v1alpha1"
)

// StagedBindingPolicy builds on top of Kubestellar's BindingPolicy to add a staging capabilities.
// A StagedBindingPolicy contains stages, each of which defines a BindingPolicy and a CEL condition
// that completes the stage when evaluated to true.
//
// The condition is checked against CombinedStatuses associated with the BindingPolicy at the active stage.
// In addition to the condition expression, a filter expression can be provided to filter the CombinedStatuses
// that are considered for the condition evaluation.
// In the future, the filtering and condition checking may be done against all resources associated with the
// workload selected by the BindingPolicy.
//
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName={sbp}
type StagedBindingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StagedBindingPolicySpec   `json:"spec,omitempty"`
	Status StagedBindingPolicyStatus `json:"status,omitempty"`
}

// StagedBindingPolicySpec defines the desired state of StagedBindingPolicy.
type StagedBindingPolicySpec struct {
	// Stages is a list of stages that define the order in which the BindingPolicies are generated.
	Stages []Stage `json:"stages"`
}

// Stage defines a stage in a StagedBindingPolicy.
type Stage struct {
	// `name` is the name of the stage.
	Name string `json:"name"`
	// `bindingPolicySpec` defines the BindingPolicy that is used in this stage.
	BindingPolicySpec v1alpha1.BindingPolicySpec `json:"bindingPolicySpec"`
	// `filter` is a CEL expression that filters the CombinedStatuses that are considered for the ConditionExpression.
	Filter v1alpha1.Expression `json:"filter"`
	// `condition` is a CEL expression that completes the stage when evaluated to true.
	Condition v1alpha1.Expression `json:"condition"`
}

// StagedBindingPolicyStatus defines the observed state of StagedBindingPolicy.
type StagedBindingPolicyStatus struct {
	// `activeStage` is the name of the active stage.
	ActiveStage string `json:"activeStage"`
}

// +kubebuilder:object:root=true

// StagedBindingPolicyList contains a list of StagedBindingPolicy.
type StagedBindingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StagedBindingPolicy `json:"items"`
}
