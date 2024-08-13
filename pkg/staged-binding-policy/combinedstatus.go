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
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) processCombinedStatus(ctx context.Context, ref string) error {
	logger := klog.FromContext(ctx)

	ns, name, err := cache.SplitMetaNamespaceKey(ref)
	if err != nil {
		return err
	}

	combinedStatus, err := c.combinedStatusLister.CombinedStatuses(ns).Get(name) //readonly
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get CombinedStatus from informer cache (ns=%v, name=%v): %w", ns, name, err)
	}

	c.resolver.NoteCombinedStatus(ctx, combinedStatus)

	logger.Info("CombinedStatus processed", "ns", ns, "name", name)
	return nil
}
