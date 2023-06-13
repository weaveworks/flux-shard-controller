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

// +kubebuilder:rbac:groups=templates.weave.works,resources=fluxshardsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=templates.weave.works,resources=fluxshardsets/status,verbs=get;update;patch
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *FluxShardSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	shardSet := templatesv1.FluxShardSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, &shardSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip reconciliation if the FluxShardSet is suspended.
	if shardSet.Spec.Suspend {
		logger.Info("Reconciliation is suspended for this FluxShardSet")
		return ctrl.Result{}, nil
	}

	// Set the value of the reconciliation request in status.
	if v, ok := fluxMeta.ReconcileAnnotationValue(shardSet.GetAnnotations()); ok {
		shardSet.Status.LastHandledReconcileAt = v
	}

	inventory, err := r.reconcileResources(ctx, &shardSet)

	if err != nil {
		templatesv1.SetFluxShardSetReadiness(&shardSet, metav1.ConditionFalse, templatesv1.ReconciliationFailedReason, err.Error())
		if err := r.patchStatus(ctx, req, shardSet.Status); err != nil {
			logger.Error(err, "failed to reconcile")
		}

		return ctrl.Result{}, err
	}

	if inventory != nil {
		templatesv1.SetReadyWithInventory(&shardSet, inventory, templatesv1.ReconciliationSucceededReason,
			fmt.Sprintf("%d shard(s) created", len(inventory.Entries)))

		if err := r.patchStatus(ctx, req, shardSet.Status); err != nil {
			templatesv1.SetFluxShardSetReadiness(&shardSet, metav1.ConditionFalse, templatesv1.ReconciliationFailedReason, err.Error())
			logger.Error(err, "failed to reconcile")
			return ctrl.Result{}, fmt.Errorf("failed to update status and inventory: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *FluxShardSetReconciler) removeResourceRefs(ctx context.Context, deletions []templatesv1.ResourceRef) error {
	logger := log.FromContext(ctx)
	for _, v := range deletions {
		d, err := deploymentFromResourceRef(v)
		if err != nil {
			return err
		}
		if err := logResourceMessage(logger, "deleting resource", d); err != nil {
			return err
		}
		if err := r.Client.Delete(ctx, d); err != nil {
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

func (r *FluxShardSetReconciler) reconcileResources(ctx context.Context, fluxShardSet *templatesv1.FluxShardSet) (*templatesv1.ResourceInventory, error) {
	logger := log.FromContext(ctx)

	srcDeploy, err := r.getSourceDeployment(ctx, fluxShardSet)
	if err != nil {
		return nil, err
	}

	generatedDeployments, err := deploys.GenerateDeployments(fluxShardSet, srcDeploy)
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployments: %w", err)
	}

	existingInventory := sets.New[templatesv1.ResourceRef]()
	if fluxShardSet.Status.Inventory != nil {
		existingInventory.Insert(fluxShardSet.Status.Inventory.Entries...)
	}

	// newInventory holds the resource refs for the generated resources.
	newInventory := sets.New[templatesv1.ResourceRef]()

	for _, newDeployment := range generatedDeployments {
		ref, err := templatesv1.ResourceRefFromObject(newDeployment)
		if err != nil {
			return nil, fmt.Errorf("failed to update inventory: %w", err)
		}

		if existingInventory.Has(ref) {
			newInventory.Insert(ref)
			// TODO if existing entries has ref, update it
			continue
		}

		if err := controllerutil.SetOwnerReference(fluxShardSet, newDeployment, r.Scheme); err != nil {
			return nil, fmt.Errorf("failed to set owner reference: %w", err)
		}

		if err := r.Client.Create(ctx, newDeployment); err != nil {
			return nil, fmt.Errorf("failed to create Deployment: %w", err)
		}
		newInventory.Insert(ref)
		if err := logResourceMessage(logger, "created new deployment", newDeployment); err != nil {
			return nil, err
		}
	}

	if fluxShardSet.Status.Inventory == nil {
		return &templatesv1.ResourceInventory{Entries: newInventory.SortedList(func(x, y templatesv1.ResourceRef) bool {
			return x.ID < y.ID
		})}, nil

	}

	// if existingEntries has more Deployments not in generated Deployments, delete and remove them from inventory
	objectsToRemove := existingInventory.Difference(newInventory)

	if err := r.removeResourceRefs(ctx, objectsToRemove.List()); err != nil {
		return nil, err
	}

	return &templatesv1.ResourceInventory{Entries: newInventory.SortedList(func(x, y templatesv1.ResourceRef) bool {
		return x.ID < y.ID
	})}, nil
}

func (r *FluxShardSetReconciler) getSourceDeployment(ctx context.Context, fluxShardSet *templatesv1.FluxShardSet) (*appsv1.Deployment, error) {
	srcDeployKey := client.ObjectKey{
		Name:      fluxShardSet.Spec.SourceDeploymentRef.Name,
		Namespace: fluxShardSet.Spec.SourceDeploymentRef.Namespace,
	}
	srcDeploy := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, srcDeployKey, srcDeploy); err != nil {
		return nil, fmt.Errorf("failed to load source deployment: %w", err)
	}

	return srcDeploy, nil
}

func (r *FluxShardSetReconciler) patchStatus(ctx context.Context, req ctrl.Request, newStatus templatesv1.FluxShardSetStatus) error {
	var set templatesv1.FluxShardSet
	if err := r.Client.Get(ctx, req.NamespacedName, &set); err != nil {
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
