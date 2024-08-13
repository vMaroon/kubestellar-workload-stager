/*
Copyright The KubeStellar Authors.

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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/vMaroon/kubestellar-workload-stager/api/community/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// StagedBindingPolicyLister helps list StagedBindingPolicies.
// All objects returned here must be treated as read-only.
type StagedBindingPolicyLister interface {
	// List lists all StagedBindingPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.StagedBindingPolicy, err error)
	// Get retrieves the StagedBindingPolicy from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.StagedBindingPolicy, error)
	StagedBindingPolicyListerExpansion
}

// stagedBindingPolicyLister implements the StagedBindingPolicyLister interface.
type stagedBindingPolicyLister struct {
	indexer cache.Indexer
}

// NewStagedBindingPolicyLister returns a new StagedBindingPolicyLister.
func NewStagedBindingPolicyLister(indexer cache.Indexer) StagedBindingPolicyLister {
	return &stagedBindingPolicyLister{indexer: indexer}
}

// List lists all StagedBindingPolicies in the indexer.
func (s *stagedBindingPolicyLister) List(selector labels.Selector) (ret []*v1alpha1.StagedBindingPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.StagedBindingPolicy))
	})
	return ret, err
}

// Get retrieves the StagedBindingPolicy from the index for a given name.
func (s *stagedBindingPolicyLister) Get(name string) (*v1alpha1.StagedBindingPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("stagedbindingpolicy"), name)
	}
	return obj.(*v1alpha1.StagedBindingPolicy), nil
}
