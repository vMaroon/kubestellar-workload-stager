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
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

func (c *Controller) syncStagedBindingPolicy(ctx context.Context, ref string) error {
	logger := klog.FromContext(ctx)

	stagedBindingPolicy, err := c.stagedBindingPolicyLister.Get(ref) //readonly
	if errors.IsNotFound(err) {
		c.resolver.DeleteStagedBindingPolicy(ctx, ref) // deletes maintained info
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get StagedBindingPolicy from informer cache (name=%v): %w", ref, err)
	}

	if err := c.resolver.NoteStagedBindingPolicy(ctx, stagedBindingPolicy); err != nil {
		return fmt.Errorf("failed to note StagedBindingPolicy (%v): %w", ref, err)
	}

	logger.Info("StagedBindingPolicy synced", "name", ref)
	return nil
}
