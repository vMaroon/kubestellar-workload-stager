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
	"time"

	"github.com/go-logr/logr"
	communityv1alpha1 "github.com/vMaroon/kubestellar-workload-stager/api/community/v1alpha1"
	sbpclient "github.com/vMaroon/kubestellar-workload-stager/pkg/generated/clientset/versioned"
	communityclient "github.com/vMaroon/kubestellar-workload-stager/pkg/generated/clientset/versioned/typed/community/v1alpha1"
	sbpinformers "github.com/vMaroon/kubestellar-workload-stager/pkg/generated/informers/externalversions"
	communityinformers "github.com/vMaroon/kubestellar-workload-stager/pkg/generated/informers/externalversions/community/v1alpha1"
	"github.com/vMaroon/kubestellar-workload-stager/pkg/generated/listers/community/v1alpha1"
	"golang.org/x/time/rate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	ksv1alpha1 "github.com/kubestellar/kubestellar/api/control/v1alpha1"
	ksclient "github.com/kubestellar/kubestellar/pkg/generated/clientset/versioned"
	controlclient "github.com/kubestellar/kubestellar/pkg/generated/clientset/versioned/typed/control/v1alpha1"
	ksinformers "github.com/kubestellar/kubestellar/pkg/generated/informers/externalversions"
	controlinformers "github.com/kubestellar/kubestellar/pkg/generated/informers/externalversions/control/v1alpha1"
	controllisters "github.com/kubestellar/kubestellar/pkg/generated/listers/control/v1alpha1"
	"github.com/kubestellar/kubestellar/pkg/util"
)

const (
	ControllerName      = "StagedBindingPolicyController"
	defaultResyncPeriod = time.Duration(0)
)

const (
	// https://github.com/kubernetes/kubernetes/blob/5d527dcf1265d7fcd0e6c8ec511ce16cc6a40699/staging/src/k8s.io/cli-runtime/pkg/genericclioptions/config_flags.go#L477
	referenceBurstUpperBound = 300
	// https://github.com/kubernetes/kubernetes/pull/105520/files
	referenceQPSUpperBound = 50.0
)

type Controller struct {
	logger                      logr.Logger
	controlClient               controlclient.ControlV1alpha1Interface     // used for Binding, BindingPolicy
	communityClient             communityclient.CommunityV1alpha1Interface // used for StagedBindingPolicy
	ksInformerFactoryStart      func(stopCh <-chan struct{})
	sbpInformerFactoryStart     func(stopCh <-chan struct{})
	bindingInformer             cache.SharedIndexInformer
	bindingLister               controllisters.BindingLister
	combinedStatusInformer      cache.SharedIndexInformer
	combinedStatusLister        controllisters.CombinedStatusLister
	stagedBindingPolicyInformer cache.SharedIndexInformer
	stagedBindingPolicyLister   v1alpha1.StagedBindingPolicyLister

	// Contains bindingPolicyRef, bindingRef, combinedStatusRef
	workqueue     workqueue.RateLimitingInterface
	initializedTs time.Time
	wdsName       string

	resolver Resolver
}

// stagedBindingPolicyRef is a workqueue item that references a StagedBindingPolicy.
type stagedBindingPolicyRef string

// bindingRef is a workqueue item that references a Binding.
type bindingRef string

// combinedStatusRef is a workqueue item that references a CombinedStatus.
type combinedStatusRef string

func NewController(parentLogger logr.Logger, wdsRestConfig *rest.Config, wdsName string) (*Controller, error) {
	logger := parentLogger.WithName(ControllerName)

	ksClient, err := ksclient.NewForConfig(wdsRestConfig)
	if err != nil {
		return nil, err
	}

	sbpClient, err := sbpclient.NewForConfig(wdsRestConfig)
	if err != nil {
		return nil, err
	}

	ksInformerFactory := ksinformers.NewSharedInformerFactory(ksClient, defaultResyncPeriod)
	sbpInformerFactory := sbpinformers.NewSharedInformerFactory(sbpClient, defaultResyncPeriod)

	return makeController(logger, ksClient.ControlV1alpha1(), sbpClient.CommunityV1alpha1(), ksInformerFactory.Start, sbpInformerFactory.Start,
		ksInformerFactory.Control().V1alpha1(), sbpInformerFactory.Community().V1alpha1(), wdsName)
}

func computeBurstFromNumGVRs(nGVRs int) int {
	burst := nGVRs
	// in case too small, fall back to default
	if burst < rest.DefaultBurst {
		return rest.DefaultBurst
	}
	// in case too large, look at some value for reference
	if burst > referenceBurstUpperBound {
		return referenceBurstUpperBound
	}
	return burst
}

func computeQPSFromNumGVRs(nGVRs int) float32 {
	qps := float32(nGVRs) / 4
	// in case too small, fall back to default
	if qps < rest.DefaultQPS {
		return rest.DefaultQPS
	}
	// in case too large, look at some value for reference
	if qps > referenceQPSUpperBound {
		return referenceQPSUpperBound
	}
	return qps
}

func makeController(logger logr.Logger,
	controlClient controlclient.ControlV1alpha1Interface,
	communityClient communityclient.CommunityV1alpha1Interface,
	ksInformerFactoryStart func(stopCh <-chan struct{}),
	sbpInformerFactoryStart func(stopCh <-chan struct{}),
	controlInformers controlinformers.Interface,
	sbpInformers communityinformers.Interface,
	wdsName string) (*Controller, error) {

	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &Controller{
		wdsName:                     wdsName,
		logger:                      logger,
		controlClient:               controlClient,
		communityClient:             communityClient,
		ksInformerFactoryStart:      ksInformerFactoryStart,
		sbpInformerFactoryStart:     sbpInformerFactoryStart,
		bindingInformer:             controlInformers.Bindings().Informer(),
		bindingLister:               controlInformers.Bindings().Lister(),
		combinedStatusInformer:      controlInformers.CombinedStatuses().Informer(),
		combinedStatusLister:        controlInformers.CombinedStatuses().Lister(),
		stagedBindingPolicyInformer: sbpInformers.StagedBindingPolicies().Informer(),
		stagedBindingPolicyLister:   sbpInformers.StagedBindingPolicies().Lister(),
		workqueue: workqueue.NewRateLimitingQueueWithConfig(ratelimiter,
			workqueue.RateLimitingQueueConfig{Name: ControllerName + "-" + wdsName}),
	}

	return controller, nil
}

// Start the controller
func (c *Controller) Start(parentCtx context.Context, workers int) error {
	logger := klog.FromContext(parentCtx).WithName(ControllerName)
	ctx := klog.NewContext(parentCtx, logger)

	if err := c.setupBindingInformer(ctx); err != nil {
		return err
	}
	if err := c.setupCombinedStatusInformer(ctx); err != nil {
		return err
	}
	if err := c.setupStagedBindingPolicyInformer(ctx); err != nil {
		return err
	}

	celEvaluator, err := newCELEvaluator()
	if err != nil {
		return err
	}

	c.resolver = NewResolver(c.combinedStatusLister, c.controlClient, c.communityClient, celEvaluator)

	c.ksInformerFactoryStart(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), c.bindingInformer.HasSynced, c.combinedStatusInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for KubeStellar informers to sync")
	}

	c.sbpInformerFactoryStart(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), c.stagedBindingPolicyInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for StagedBindingPolicy informers to sync")
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- c.run(ctx, workers)
	}()

	// check for errors at startup, after all started we let it continue
	// so we can start the controller-runtime manager
	select {
	case err := <-errChan:
		return err
	case <-time.After(3 * time.Second):
		return nil
	}
}

// Invoked by Start() to run the controller
func (c *Controller) run(ctx context.Context, workers int) error {
	defer c.workqueue.ShutDown()
	c.logger.Info("Starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		logger := c.logger.WithName(fmt.Sprintf("worker-%d", i))
		workerCtx := klog.NewContext(ctx, logger)
		go wait.UntilWithContext(workerCtx, c.runWorker, time.Second)
	}

	c.logger.Info("Started workers")
	c.initializedTs = time.Now()

	<-ctx.Done()
	c.logger.Info("Shutting down workers")

	return nil
}

func (c *Controller) setupBindingInformer(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	_, err := c.bindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			bdg := obj.(*ksv1alpha1.Binding)
			logger.V(5).Info("Enqueuing reference to Binding because of informer add event", "name", bdg.Name, "resourceVersion", bdg.ResourceVersion)
			c.workqueue.Add(bindingRef(bdg.Name))
		},
		UpdateFunc: func(old, new interface{}) {
			oldBdg := old.(*ksv1alpha1.Binding)
			newBdg := new.(*ksv1alpha1.Binding)
			if oldBdg.Generation != newBdg.Generation {
				logger.V(5).Info("Enqueuing reference to Binding because of informer update event", "name", newBdg.Name, "resourceVersion", newBdg.ResourceVersion)
				c.workqueue.Add(bindingRef(newBdg.Name))
			}
		},
		DeleteFunc: func(obj interface{}) {
			if typed, is := obj.(cache.DeletedFinalStateUnknown); is {
				obj = typed.Obj
			}
			bdg := obj.(*ksv1alpha1.Binding)
			logger.V(5).Info("Enqueuing reference to Binding because of informer delete event", "name", bdg.Name)
			c.workqueue.Add(bindingRef(bdg.Name))
		},
	})
	if err != nil {
		c.logger.Error(err, "failed to add bindingpolicies informer event handler")
		return err
	}
	return nil
}

func (c *Controller) setupCombinedStatusInformer(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	_, err := c.combinedStatusInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cs := obj.(*ksv1alpha1.CombinedStatus)
			logger.V(5).Info("Enqueuing reference to CombinedStatus because of informer add event", "name", cs.Name, "resourceVersion", cs.ResourceVersion)
			c.enqueueCombinedStatus(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			oldCS := old.(*ksv1alpha1.CombinedStatus)
			newCS := new.(*ksv1alpha1.CombinedStatus)
			if oldCS.Generation != newCS.Generation {
				logger.V(5).Info("Enqueuing reference to CombinedStatus because of informer update event", "name", newCS.Name, "resourceVersion", newCS.ResourceVersion)
				c.enqueueCombinedStatus(new)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if typed, is := obj.(cache.DeletedFinalStateUnknown); is {
				obj = typed.Obj
			}
			cs := obj.(*ksv1alpha1.CombinedStatus)
			logger.V(5).Info("Enqueuing reference to CombinedStatus because of informer delete event", "name", cs.Name)
			c.enqueueCombinedStatus(obj)
		},
	})
	if err != nil {
		c.logger.Error(err, "failed to add combinedstatuses informer event handler")
		return err
	}
	return nil
}

func (c *Controller) enqueueCombinedStatus(obj any) {
	ref, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to get key for CombinedStatus", "obj", obj)
		return
	}

	c.workqueue.Add(combinedStatusRef(ref))
}

func (c *Controller) setupStagedBindingPolicyInformer(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	_, err := c.stagedBindingPolicyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sbp := obj.(*communityv1alpha1.StagedBindingPolicy)
			logger.V(5).Info("Enqueuing reference to StagedBindingPolicy because of informer add event", "name", sbp.Name, "resourceVersion", sbp.ResourceVersion)
			c.workqueue.Add(stagedBindingPolicyRef(sbp.Name))
		},
		UpdateFunc: func(old, new interface{}) {
			oldSBP := old.(*communityv1alpha1.StagedBindingPolicy)
			newSBP := new.(*communityv1alpha1.StagedBindingPolicy)
			if oldSBP.Generation != newSBP.Generation {
				logger.V(5).Info("Enqueuing reference to StagedBindingPolicy because of informer update event", "name", newSBP.Name, "resourceVersion", newSBP.ResourceVersion)
				c.workqueue.Add(stagedBindingPolicyRef(newSBP.Name))
			}
		},
		DeleteFunc: func(obj interface{}) {
			if typed, is := obj.(cache.DeletedFinalStateUnknown); is {
				obj = typed.Obj
			}
			sbp := obj.(*communityv1alpha1.StagedBindingPolicy)
			logger.V(5).Info("Enqueuing reference to StagedBindingPolicy because of informer delete event", "name", sbp.Name)
			c.workqueue.Add(stagedBindingPolicyRef(sbp.Name))
		},
	})
	if err != nil {
		c.logger.Error(err, "failed to add stagedbindingpolicies informer event handler")
		return err
	}
	return nil
}

func shouldSkipUpdate(old, new interface{}) bool {
	oldMObj := old.(metav1.Object)
	newMObj := new.(metav1.Object)
	// do not enqueue update events for objects that have not changed
	if newMObj.GetResourceVersion() == oldMObj.GetResourceVersion() {
		return true
	}

	return false
}

// Event handler: enqueues the objects to be processed
// At this time it is very simple, more complex processing might be required
// here.
func (c *Controller) handleObject(obj any, resource string, eventType string) {
	wasDeletedFinalStateUnknown := false
	switch typed := obj.(type) {
	case cache.DeletedFinalStateUnknown:
		obj = typed.Obj
		wasDeletedFinalStateUnknown = true
	}
	c.logger.V(4).Info("Got object event", "eventType", eventType,
		"wasDeletedFinalStateUnknown", wasDeletedFinalStateUnknown, "obj", util.RefToRuntimeObj(obj.(runtime.Object)),
		"resource", resource)

	c.enqueueObject(obj, resource)
}

// enqueueObject converts an object into an ObjectIdentifier struct which is
// then put onto the work queue.
func (c *Controller) enqueueObject(obj interface{}, resource string) {
	objIdentifier := util.IdentifierForObject(obj.(util.MRObject), resource)
	c.enqueueObjectIdentifier(objIdentifier)
}

func (c *Controller) enqueueObjectIdentifier(objIdentifier util.ObjectIdentifier) {
	c.workqueue.Add(objIdentifier)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem reads a single work item off the workqueue and
// attempt to process it by calling the reconcile.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	logger := klog.FromContext(ctx)
	item, shutdown := c.workqueue.Get()
	if shutdown {
		logger.V(1).Info("Worker is done")
		return false
	}
	logger.V(4).Info("Dequeued", "item", item, "type", fmt.Sprintf("%T", item))

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func() error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(item)
		// Run the reconciler, passing it the full object identifier
		if err := c.reconcile(ctx, item); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(item)
			return fmt.Errorf("error reconciling object (identifier: %#v, type: %T): %s, requeuing", item, item, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(item)
		logger.V(4).Info("Successfully reconciled", "objectIdentifier", item, "type", fmt.Sprintf("%T", item))
		return nil
	}()

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) reconcile(ctx context.Context, item any) error {
	logger := klog.FromContext(ctx)

	switch item := item.(type) {
	case stagedBindingPolicyRef:
		return c.syncStagedBindingPolicy(ctx, string(item))
	case bindingRef:
		return c.processBinding(ctx, string(item))
	case combinedStatusRef:
		return c.processCombinedStatus(ctx, string(item))
	}

	logger.Error(nil, "Impossible workqueue entry", "type", fmt.Sprintf("%T", item), "value", item)
	return nil
}

func getObject(lister cache.GenericLister, namespace, name string) (runtime.Object, error) {
	if namespace != "" {
		return lister.ByNamespace(namespace).Get(name)
	}
	return lister.Get(name)
}

func isBeingDeleted(obj runtime.Object) bool {
	mObj := obj.(metav1.Object)
	return mObj.GetDeletionTimestamp() != nil
}
