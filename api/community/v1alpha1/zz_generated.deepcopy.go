//go:build !ignore_autogenerated

/*
Copyright  The KubeStellar Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	controlv1alpha1 "github.com/kubestellar/kubestellar/api/control/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Stage) DeepCopyInto(out *Stage) {
	*out = *in
	if in.ClusterSelectors != nil {
		in, out := &in.ClusterSelectors, &out.ClusterSelectors
		*out = make([]v1.LabelSelector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Filter != nil {
		in, out := &in.Filter, &out.Filter
		*out = new(controlv1alpha1.Expression)
		**out = **in
	}
	if in.Condition != nil {
		in, out := &in.Condition, &out.Condition
		*out = new(controlv1alpha1.Expression)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Stage.
func (in *Stage) DeepCopy() *Stage {
	if in == nil {
		return nil
	}
	out := new(Stage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StagedBindingPolicy) DeepCopyInto(out *StagedBindingPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StagedBindingPolicy.
func (in *StagedBindingPolicy) DeepCopy() *StagedBindingPolicy {
	if in == nil {
		return nil
	}
	out := new(StagedBindingPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *StagedBindingPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StagedBindingPolicyList) DeepCopyInto(out *StagedBindingPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]StagedBindingPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StagedBindingPolicyList.
func (in *StagedBindingPolicyList) DeepCopy() *StagedBindingPolicyList {
	if in == nil {
		return nil
	}
	out := new(StagedBindingPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *StagedBindingPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StagedBindingPolicySpec) DeepCopyInto(out *StagedBindingPolicySpec) {
	*out = *in
	if in.Downsync != nil {
		in, out := &in.Downsync, &out.Downsync
		*out = make([]controlv1alpha1.DownsyncPolicyClause, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Stages != nil {
		in, out := &in.Stages, &out.Stages
		*out = make([]Stage, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StagedBindingPolicySpec.
func (in *StagedBindingPolicySpec) DeepCopy() *StagedBindingPolicySpec {
	if in == nil {
		return nil
	}
	out := new(StagedBindingPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StagedBindingPolicyStatus) DeepCopyInto(out *StagedBindingPolicyStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StagedBindingPolicyStatus.
func (in *StagedBindingPolicyStatus) DeepCopy() *StagedBindingPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(StagedBindingPolicyStatus)
	in.DeepCopyInto(out)
	return out
}
