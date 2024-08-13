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

func (c *Controller) processBinding(ctx context.Context, ref string) error {
	logger := klog.FromContext(ctx)

	binding, err := c.bindingLister.Get(ref) //readonly
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get Binding from informer cache (name=%v): %w", ref, err)
	}

	c.resolver.NoteBinding(ctx, binding)

	logger.Info("Binding processed", "name", ref)
	return nil
}
