/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cli-utils/pkg/object"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxMeta "github.com/fluxcd/pkg/apis/meta"
	"github.com/gitops-tools/pkg/sets"
	"github.com/go-logr/logr"
	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	deploys "github.com/weaveworks/flux-shard-controller/internal/deploys"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var accessor = meta.NewAccessor()

// FluxShardSetReconciler reconciles a FluxShardSet object
type FluxShardSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=templates.weave.works,resources=fluxshardsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=templates.weave.works,resources=fluxshardsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=templates.weave.works,resources=fluxshardsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FluxShardSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *FluxShardSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	shardSet := templatesv1.FluxShardSet{}
	if err := r.Get(ctx, req.NamespacedName, &shardSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Reconciling shardSet",
		"type", shardSet.Spec.Type,
		"shards", shardSet.Spec.Shards,
	)

	// Add finalizer first if it doesn't exist to avoid the race condition
	// between init and delete.
	if !controllerutil.ContainsFinalizer(&shardSet, templatesv1.FluxShardSetFinalizer) {
		controllerutil.AddFinalizer(&shardSet, templatesv1.FluxShardSetFinalizer)

		return ctrl.Result{Requeue: true}, r.Update(ctx, &shardSet)
	}

	// Skip reconciliation if the FluxShardSet is suspended.
	if shardSet.Spec.Suspend {
		logger.Info("Reconciliation is suspended for this FluxShardSet")
		return ctrl.Result{}, nil
	}

	k8sClient := r.Client

	if !shardSet.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &shardSet, k8sClient)
	}

	// Set the value of the reconciliation request in status.
	if v, ok := fluxMeta.ReconcileAnnotationValue(shardSet.GetAnnotations()); ok {
		shardSet.Status.LastHandledReconcileAt = v
	}

	inventory, _, err := r.reconcileResources(ctx, k8sClient, &shardSet, req)
	// inventory, requeue, _ := r.reconcileResources(ctx, k8sClient, &shardSet, req)

	if err != nil {
		templatesv1.SetFluxShardSetReadiness(&shardSet, metav1.ConditionFalse, templatesv1.ReconciliationFailedReason, err.Error())
		if err := r.patchStatus(ctx, req, shardSet.Status); err != nil {
			logger.Error(err, "failed to reconcile")
		}

		return ctrl.Result{}, err
	}

	if inventory != nil {
		templatesv1.SetReadyWithInventory(&shardSet, inventory, templatesv1.ReconciliationSucceededReason,
			fmt.Sprintf("%d resources created", len(inventory.Entries)))

		if err := r.patchStatus(ctx, req, shardSet.Status); err != nil {
			templatesv1.SetFluxShardSetReadiness(&shardSet, metav1.ConditionFalse, templatesv1.ReconciliationFailedReason, err.Error())
			logger.Error(err, "failed to reconcile")
			return ctrl.Result{}, fmt.Errorf("failed to update status and inventory: %w", err)
		}
	}

	// return ctrl.Result{RequeueAfter: requeue}, nil
	return ctrl.Result{}, nil
}

func (r *FluxShardSetReconciler) finalize(ctx context.Context, shardset *templatesv1.FluxShardSet, k8sClient client.Client) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("finalizing resources")

	if !shardset.Spec.Suspend &&
		shardset.Status.Inventory != nil &&
		shardset.Status.Inventory.Entries != nil {

		if err := r.removeResourceRefs(ctx, k8sClient, shardset.Status.Inventory.Entries); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("cleaned resources")
	}

	logger.Info("removing the finalizer")
	// Remove our finalizer from the list and update it
	controllerutil.RemoveFinalizer(shardset, templatesv1.FluxShardSetFinalizer)
	return ctrl.Result{}, r.Update(ctx, shardset)
}

func (r *FluxShardSetReconciler) removeResourceRefs(ctx context.Context, k8sClient client.Client, deletions []templatesv1.ResourceRef) error {
	logger := log.FromContext(ctx)
	for _, v := range deletions {
		d, err := deploymentFromResourceRef(v)
		if err != nil {
			return err
		}
		if err := logResourceMessage(logger, "deleting resource", d); err != nil {
			return err
		}
		if err := k8sClient.Delete(ctx, d); err != nil {
			return fmt.Errorf("failed to delete %v: %w", d, err)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FluxShardSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&templatesv1.FluxShardSet{}).
		Complete(r)
}

func (r *FluxShardSetReconciler) reconcileResources(ctx context.Context, k8sClient client.Client, fluxShardSet *templatesv1.FluxShardSet, req ctrl.Request) (*templatesv1.ResourceInventory, time.Duration, error) {
	// logger := log.FromContext(ctx)
	deps := &appsv1.DeploymentList{}
	err := k8sClient.List(ctx, deps, &client.ListOptions{Namespace: req.Namespace})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list current deployments: %w", err)
	}

	generatedDeployments := []appsv1.Deployment{}
	for i, _ := range deps.Items {
		dep := deps.Items[i]
		if _, isShardDeployment := dep.ObjectMeta.Labels["templates.weave.works/shard-set"]; isShardDeployment {
			continue
		}
		newDeployments, err := deploys.GenerateDeployments(fluxShardSet, &dep)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to generate deployments: %w", err)
		}
		for _, d := range newDeployments {
			generatedDeployments = append(generatedDeployments, *d)
		}
	}

	existingEntries := sets.New[templatesv1.ResourceRef]()
	if fluxShardSet.Status.Inventory != nil {
		existingEntries.Insert(fluxShardSet.Status.Inventory.Entries...)
	}

	entries := sets.New[templatesv1.ResourceRef]()
	for _, newDeployment := range generatedDeployments {
		ref, err := templatesv1.ResourceRefFromObject(&newDeployment)
		if err != nil {
			return nil, templatesv1.NoRequeueInterval, fmt.Errorf("failed to update inventory: %w", err)
		}
		entries.Insert(ref)

		if existingEntries.Has(ref) {
			continue
			// TODO if existing entries has ref, update it

		}

		// if err := logResourceMessage(logger, "creating new resource", newDeployment); err != nil {
		// 	// TODO return requeue time
		// 	return nil, templatesv1.NoRequeueInterval, err
		// }

		if err := controllerutil.SetOwnerReference(fluxShardSet, &newDeployment, r.Scheme); err != nil {
			return nil, templatesv1.NoRequeueInterval, fmt.Errorf("failed to set owner reference: %w", err)
		}

		if err := k8sClient.Create(ctx, &newDeployment); err != nil {
			// TODO return requeue time
			return nil, templatesv1.NoRequeueInterval, fmt.Errorf("failed to create Deployment: %w", err)
		}
	}

	if fluxShardSet.Status.Inventory == nil {
		// TODO return requeue time
		return &templatesv1.ResourceInventory{Entries: entries.SortedList(func(x, y templatesv1.ResourceRef) bool {
			return x.ID < y.ID
		})}, templatesv1.NoRequeueInterval, nil

	}
	// if existingEntries has more Deployments not in generated Deployments, delete and remove them from inventory
	objectsToRemove := existingEntries.Difference(entries)
	if err := r.removeResourceRefs(ctx, k8sClient, objectsToRemove.List()); err != nil {
		return nil, templatesv1.NoRequeueInterval, err
	}
	// TODO calculateInterval
	// requeueAfter, err := calculateInterval(fluxShardSet, generatedDeployments)
	// if err != nil {
	// 	return nil, nil, fmt.Errorf("failed to calculate requeue interval: %w", err)
	// }

	return &templatesv1.ResourceInventory{Entries: entries.SortedList(func(x, y templatesv1.ResourceRef) bool {
		return x.ID < y.ID
	})}, templatesv1.NoRequeueInterval, nil

}

func (r *FluxShardSetReconciler) patchStatus(ctx context.Context, req ctrl.Request, newStatus templatesv1.FluxShardSetStatus) error {
	var set templatesv1.FluxShardSet
	if err := r.Get(ctx, req.NamespacedName, &set); err != nil {
		return err
	}

	patch := client.MergeFrom(set.DeepCopy())
	set.Status = newStatus

	return r.Status().Patch(ctx, &set, patch)
}

func deploymentFromResourceRef(ref templatesv1.ResourceRef) (*appsv1.Deployment, error) {
	objMeta, err := object.ParseObjMetadata(ref.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse object ID %s: %w", ref.ID, err)
	}
	d := appsv1.Deployment{}

	d.Namespace = objMeta.Namespace
	d.Name = objMeta.Name
	return &d, nil

}

func logResourceMessage(logger logr.Logger, msg string, obj runtime.Object) error {
	// TODO enhance
	namespace, err := accessor.Namespace(obj)
	if err != nil {
		return err
	}
	name, err := accessor.Name(obj)
	if err != nil {
		return err
	}
	kind, err := accessor.Kind(obj)
	if err != nil {
		return err
	}

	logger.Info(msg, "objNamespace", namespace, "objName", name, "kind", kind)

	return nil
}
