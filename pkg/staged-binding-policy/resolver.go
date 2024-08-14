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

package staged_binding_policy

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"strconv"
	"sync"

	"github.com/vMaroon/kubestellar-workload-stager/api/community/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	v1alpha12 "github.com/kubestellar/kubestellar/api/control/v1alpha1"
	controlclient "github.com/kubestellar/kubestellar/pkg/generated/clientset/versioned/typed/control/v1alpha1"
	controllisters "github.com/kubestellar/kubestellar/pkg/generated/listers/control/v1alpha1"
	"github.com/kubestellar/kubestellar/pkg/util"
)

const (
	// CombinedStatusLabelBindingPolicy is the label key for the binding policy associated with the combined status.
	CombinedStatusLabelBindingPolicy = "status.kubestellar.io/binding-policy"
	// CombinedStatusLabelGroup is the label key for the API group of the workload object associated with the combined status.
	CombinedStatusLabelGroup = "status.kubestellar.io/api-group"
	// CombinedStatusLabelResource is the label key for the API resource of the workload object associated with the combined status.
	CombinedStatusLabelResource = "status.kubestellar.io/resource"
	// CombinedStatusLabelName is the label key for the name of the workload object associated with the combined status.
	CombinedStatusLabelName = "status.kubestellar.io/name"
	// CombinedStatusLabelNamespace is the label key for the namespace of the workload object associated with the combined status.
	CombinedStatusLabelNamespace = "status.kubestellar.io/namespace"
)

// Resolver is an interface that defines the methods for the staged binding
// policy resolver.
// The resolver is responsible for tracking the staged binding policies and
// their stages. In its current state, the resolver has a simple implementation
// that stages up the binding policies based on the filters and conditions
// defined in the stages.
// Several optimizations and improvements are required to make the resolver a
// proper implementation.
type Resolver interface {
	// NoteStagedBindingPolicy notes a staged binding policy, ensuring that it
	// is tracked and its active stage is deployed as a binding policy.
	NoteStagedBindingPolicy(context.Context, *v1alpha1.StagedBindingPolicy) error
	// DeleteStagedBindingPolicy deletes a staged binding policy from the resolver.
	DeleteStagedBindingPolicy(context.Context, string)
	// NoteBinding notes a binding if associated with a staged binding policy.
	// The binding's spec is used to set the space of objects that are checked
	// against the condition of the active stage, after filtering.
	NoteBinding(context.Context, *v1alpha12.Binding)
	// NoteCombinedStatus notes a combined status if associated with a staged
	// binding policy. Upon trigger, all staged binding policies associated with
	// the combined status are checked for staging up.
	NoteCombinedStatus(context.Context, *v1alpha12.CombinedStatus)
}

type resolver struct {
	combinedStatusLister controllisters.CombinedStatusLister
	ksControlClient      controlclient.ControlV1alpha1Interface
	celEvaluator         *celEvaluator

	sync.Mutex
	stagedBindingPolicies map[string]*stagedBindingPolicyData
}

type stagedBindingPolicyData struct {
	uid    types.UID
	stages []v1alpha1.Stage

	activeIndex            int
	activeBindingPolicyUID types.UID

	bindingSpec v1alpha12.BindingSpec
}

func NewResolver(combinedStatusLister controllisters.CombinedStatusLister,
	controlclient controlclient.ControlV1alpha1Interface, celEvaluator *celEvaluator) Resolver {
	return &resolver{
		combinedStatusLister:  combinedStatusLister,
		ksControlClient:       controlclient,
		celEvaluator:          celEvaluator,
		stagedBindingPolicies: make(map[string]*stagedBindingPolicyData),
	}
}

// NoteStagedBindingPolicy notes a staged binding policy, ensuring that it
// is tracked and its active stage is deployed as a binding policy.
func (r *resolver) NoteStagedBindingPolicy(ctx context.Context, sbp *v1alpha1.StagedBindingPolicy) error {
	r.Lock()
	defer r.Unlock()

	activeStage := 0
	if sbp.Status.ActiveStage != "" {
		if numeric, err := strconv.ParseInt(sbp.Status.ActiveStage, 10, 64); err != nil {
			activeStage = int(numeric)
		}
	}

	r.stagedBindingPolicies[sbp.Name] = &stagedBindingPolicyData{
		uid:         sbp.UID,
		stages:      sbp.Spec.Stages,
		activeIndex: activeStage,
	}

	// ensure a bp for the active stage
	return r.ensureActiveStageLocked(ctx, sbp.Name, r.stagedBindingPolicies[sbp.Name])
}

// DeleteStagedBindingPolicy deletes a staged binding policy from the resolver.
func (r *resolver) DeleteStagedBindingPolicy(ctx context.Context, sbpName string) {
	r.Lock()
	defer r.Unlock()

	delete(r.stagedBindingPolicies, sbpName)
}

// NoteBinding notes a binding if associated with a staged binding policy.
// The binding's spec is used to set the space of objects that are checked
// against the condition of the active stage, after filtering.
func (r *resolver) NoteBinding(ctx context.Context, binding *v1alpha12.Binding) {
	r.Lock()
	defer r.Unlock()

	sbpData, ok := r.stagedBindingPolicies[binding.Name]
	if !ok {
		return // no staged binding policy
	}

	var bpUID types.UID
	if len(binding.ObjectMeta.OwnerReferences) > 0 {
		bpUID = binding.ObjectMeta.OwnerReferences[0].UID // should be that of the owning BP
	}

	if sbpData.activeBindingPolicyUID != bpUID {
		return // not the active binding policy
	}

	sbpData.bindingSpec = binding.Spec // readonly

	// attempt to stage up
	r.attemptConditionLocked(ctx, binding.Name)
}

// NoteCombinedStatus notes a combined status if associated with a staged
// binding policy. Upon trigger, all staged binding policies associated with
// the combined status are checked for staging up.
func (r *resolver) NoteCombinedStatus(ctx context.Context, combinedStatus *v1alpha12.CombinedStatus) {
	r.Lock()
	defer r.Unlock()

	associatedBindingName := combinedStatus.Labels[CombinedStatusLabelBindingPolicy]
	if associatedBindingName == "" {
		return // malformed
	}

	// attempt to stage up relevant resolver entries
	for sbpName := range r.stagedBindingPolicies {
		if sbpName != associatedBindingName {
			continue
		}

		r.attemptConditionLocked(ctx, sbpName)
	}
}

func (r *resolver) attemptConditionLocked(ctx context.Context, sbpName string) {
	sbpData, ok := r.stagedBindingPolicies[sbpName]
	if !ok {
		return // no staged binding policy
	}

	if sbpData.activeIndex >= len(sbpData.stages) {
		return // no more stages
	}

	if sbpData.stages[sbpData.activeIndex].Condition == nil {
		return // no condition to satisfy
	}

	filteredObjIdentifiers := filterWorkloadForSBP(ctx, r.celEvaluator, &sbpData.bindingSpec.Workload,
		sbpData.stages[sbpData.activeIndex].Filter)

	// try to match the condition on all combinedstatuses associated with the active binding + filtered objects
	for objIdentifier := range filteredObjIdentifiers {
		// build label selectors to match the combined status
		groupReq, _ := labels.NewRequirement(CombinedStatusLabelGroup, selection.Equals,
			[]string{objIdentifier.GVK.Group})
		resourceReq, _ := labels.NewRequirement(CombinedStatusLabelResource, selection.Equals,
			[]string{objIdentifier.Resource})
		namespaceReq, _ := labels.NewRequirement(CombinedStatusLabelNamespace, selection.Equals,
			[]string{objIdentifier.ObjectName.Namespace})
		nameReq, _ := labels.NewRequirement(CombinedStatusLabelName, selection.Equals,
			[]string{objIdentifier.ObjectName.Name})
		bindingPolicyReq, _ := labels.NewRequirement(CombinedStatusLabelBindingPolicy, selection.Equals,
			[]string{sbpName})

		selector := labels.NewSelector().
			Add(*groupReq).
			Add(*resourceReq).
			Add(*namespaceReq).
			Add(*nameReq).
			Add(*bindingPolicyReq)

		combinedStatuses, err := r.combinedStatusLister.List(selector)
		if err != nil || len(combinedStatuses) == 0 {
			return // missing combined status, cannot satisfy full condition
		}

		// evaluate the condition
		serializedCombinedStatus, err := serializeObject(combinedStatuses[0])
		if err != nil {
			return
		}

		if testExpression(ctx, r.celEvaluator, *sbpData.stages[sbpData.activeIndex].Condition, map[string]interface{}{
			"obj": serializedCombinedStatus, // assuming one match, TODO: trim down to relevant match
		}) == false {
			return // condition not satisfied
		}
	}

	// condition satisfied, move to next stage
	r.stageUpLocked(ctx, sbpName)
}

func (r *resolver) stageUpLocked(ctx context.Context, sbpName string) {
	sbpData, ok := r.stagedBindingPolicies[sbpName]
	if !ok {
		return // no staged binding policy
	}

	if sbpData.activeIndex >= len(sbpData.stages) {
		return // no more stages
	}

	logger := klog.FromContext(ctx)
	sbpData.activeIndex++

	// ensure a bp for the active stage
	if err := r.ensureActiveStageLocked(ctx, sbpName, sbpData); err != nil {
		logger.Error(err, "error ensuring active stage", "sbp", sbpName)
		sbpData.activeIndex-- // revert

		return
	}

	logger.Info("staged up", "sbp", sbpName, "stage", sbpData.activeIndex)
}

func (r *resolver) ensureActiveStageLocked(ctx context.Context, sbpName string,
	sbpData *stagedBindingPolicyData) error {
	// ensure a bp for the active stage
	bpEcho, err := r.ksControlClient.BindingPolicies().Get(ctx, sbpName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			bpEcho, err := r.ksControlClient.BindingPolicies().Create(ctx, &v1alpha12.BindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: sbpName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: v1alpha1.GroupVersion.String(),
							Kind:       "StagedBindingPolicy",
							Name:       sbpName,
							UID:        sbpData.uid,
						}}},
				Spec: sbpData.stages[sbpData.activeIndex].BindingPolicySpec,
			}, metav1.CreateOptions{})

			if err != nil {
				return err
			}

			sbpData.activeBindingPolicyUID = bpEcho.UID
			return nil
		}
	}

	bpEcho, err = r.ksControlClient.BindingPolicies().Update(ctx, &v1alpha12.BindingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: sbpName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       "StagedBindingPolicy",
					Name:       sbpName,
					UID:        sbpData.uid,
				}},
			ResourceVersion: bpEcho.ResourceVersion,
		},
		Spec: sbpData.stages[sbpData.activeIndex].BindingPolicySpec,
	}, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	sbpData.activeBindingPolicyUID = bpEcho.UID
	return nil
}

func filterWorkloadForSBP(ctx context.Context, celEvaluator *celEvaluator, workload *v1alpha12.DownsyncObjectClauses,
	expression *v1alpha12.Expression) sets.Set[util.ObjectIdentifier] {
	filtered := sets.New[util.ObjectIdentifier]()

	for _, downsyncClause := range workload.ClusterScope {
		if expression != nil && testExpression(ctx, celEvaluator, *expression, map[string]interface{}{
			"downsyncClause": clusterScopeDownsyncObjectToMap(downsyncClause),
		}) == false {
			continue
		}

		filtered.Insert(util.ObjectIdentifier{
			GVK: schema.GroupVersionKind{
				Group: downsyncClause.Group,    // no version
				Kind:  downsyncClause.Resource, // works for our case
			},
			Resource: downsyncClause.Resource,
			ObjectName: cache.ObjectName{
				Name: downsyncClause.Name,
			}})
	}

	for _, downsyncClause := range workload.NamespaceScope {
		if expression != nil && testExpression(ctx, celEvaluator, *expression, map[string]interface{}{
			"downsyncClause": namespaceScopeDownsyncObjectToMap(downsyncClause),
		}) == false {
			continue
		}

		filtered.Insert(util.ObjectIdentifier{
			GVK: schema.GroupVersionKind{
				Group: downsyncClause.Group,    // no version
				Kind:  downsyncClause.Resource, // works for our case
			},
			Resource: downsyncClause.Resource,
			ObjectName: cache.ObjectName{
				Name:      downsyncClause.Name,
				Namespace: downsyncClause.Namespace,
			}})
	}

	return filtered
}
func clusterScopeDownsyncObjectToMap(downsyncClause v1alpha12.ClusterScopeDownsyncClause) map[string]interface{} {
	return map[string]interface{}{
		"group":           downsyncClause.Group,
		"version":         downsyncClause.Version,
		"resource":        downsyncClause.Resource,
		"name":            downsyncClause.Name,
		"resourceVersion": downsyncClause.ResourceVersion,
	}
}

func namespaceScopeDownsyncObjectToMap(downsyncClause v1alpha12.NamespaceScopeDownsyncClause) map[string]interface{} {
	return map[string]interface{}{
		"group":           downsyncClause.Version,
		"version":         downsyncClause.Version,
		"resource":        downsyncClause.Resource,
		"name":            downsyncClause.Name,
		"namespace":       downsyncClause.Namespace,
		"resourceVersion": downsyncClause.ResourceVersion,
	}
}

// serializeObject converts a runtime.Object to an unstructured map.
func serializeObject(obj runtime.Object) (map[string]interface{}, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func testExpression(ctx context.Context, celEvaluator *celEvaluator, expression v1alpha12.Expression,
	objMap map[string]interface{}) bool {
	logger := klog.FromContext(ctx)
	test, err := celEvaluator.Evaluate(expression, objMap)

	if err != nil {
		logger.Error(err, "error evaluating expression", "expression", expression,
			"object", objMap)
		return false
	}

	if test.Type().TypeName() != "bool" || !test.Value().(bool) {
		return false
	}

	return true
}
