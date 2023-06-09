package controller

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"github.com/weaveworks/flux-shard-controller/test"
)

func TestReconciliation(t *testing.T) {
	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			"testdata/crds",
		},
	}
	testEnv.ControlPlane.GetAPIServer().Configure().Append("--authorization-mode=RBAC")

	cfg, err := testEnv.Start()
	test.AssertNoError(t, err)
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop the test environment: %s", err)
		}
	}()

	scheme := runtime.NewScheme()
	// This deliberately only sets up the scheme for the core scheme + the
	// FluxShardSet templating scheme.
	test.AssertNoError(t, clientgoscheme.AddToScheme(scheme))
	test.AssertNoError(t, templatesv1.AddToScheme(scheme))

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	test.AssertNoError(t, err)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	test.AssertNoError(t, err)

	reconciler := &FluxShardSetReconciler{
		Client: k8sClient,
		Scheme: scheme,
	}

	test.AssertNoError(t, reconciler.SetupWithManager(mgr))
	test.AssertNoError(t, k8sClient.Create(context.TODO(), test.NewNamespace("test-ns")))

	t.Run("reconciling creation of new deployment with shard", func(t *testing.T) {
		ctx := context.TODO()
		// Create shard set and src deployment
		shardset := createAndReconcileToFinalizedState(t, k8sClient, reconciler, makeTestFluxShardSet(t, func(shardset *templatesv1.FluxShardSet) {
			shardset.Spec.Type = "kustomize"
			shardset.Spec.Shards = append(shardset.Spec.Shards, templatesv1.ShardSpec{
				Name: "shard-1",
			})

		}))
		defer deleteFluxShardSetAndFinalize(t, k8sClient, reconciler, shardset)

		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Annotations = map[string]string{}
			d.ObjectMeta.Name = "kustomize-controller"
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		reconciler.Create(ctx, srcDeployment)

		// Reconcile
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
		test.AssertNoError(t, err)

		// Check fluxshardset
		updated := &templatesv1.FluxShardSet{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shardset), updated))

		wantDeployment := test.MakeTestDeployment(nsn("default", "shard-1-kustomize-controller"), func(d *appsv1.Deployment) {
			d.Annotations = map[string]string{}
			d.ObjectMeta.Labels = map[string]string{
				"templates.weave.works/shard-set": "test-shard-set",
				"app.kubernetes.io/managed-by":    "flux-shard-controller",
			}
			d.ObjectMeta.Name = "shard-1-kustomize-controller"
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
		})
		want := []runtime.Object{
			wantDeployment,
		}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(wantDeployment), wantDeployment))

		// Check inventory updated with fluxshardset and new deployment(want) and condition of number of resources created
		test.AssertInventoryHasItems(t, updated, want...)
		assertFluxShardSetCondition(t, updated, meta.ReadyCondition, "1 resources created")

		// Check deployments existing include the new deployment
		assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller")
	})

	t.Run("Delete resources when removing shard from fluxshardset shards", func(t *testing.T) {
		ctx := context.TODO()
		// Create shard set and src deployment
		shardset := createAndReconcileToFinalizedState(t, k8sClient, reconciler, makeTestFluxShardSet(t, func(shardset *templatesv1.FluxShardSet) {
			shardset.Spec.Type = "kustomize"
			shardset.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
				{
					Name: "shard-2",
				},
			}

		}))
		defer deleteFluxShardSetAndFinalize(t, k8sClient, reconciler, shardset)

		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Annotations = map[string]string{}
			d.ObjectMeta.Name = "kustomize-controller"
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		reconciler.Create(ctx, srcDeployment)

		// Reconcile
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
		test.AssertNoError(t, err)

		// Check fluxshardset
		updated := &templatesv1.FluxShardSet{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shardset), updated))
		assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller", "shard-2-kustomize-controller")

		// Update shard set by removing shard-2
		shardset.Spec.Shards = shardset.Spec.Shards[:1]
		reconcileAndAssertFinalizerExists(t, k8sClient, reconciler, shardset)
		// Check deployment for shard-1 exists and deployment for shard-2 is deleted
		assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller")
		assertDeploymentsDontExist(t, k8sClient, "default", "shard-2-kustomize-controller")
	})

	t.Run("Create new deployments with new shard names and delete old deployments after removing shard names", func(t *testing.T) {
		ctx := context.TODO()
		// Create shard set and src deployment
		shardset := createAndReconcileToFinalizedState(t, k8sClient, reconciler, makeTestFluxShardSet(t, func(shardset *templatesv1.FluxShardSet) {
			shardset.Spec.Type = "kustomize"
			shardset.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-a",
				},
				{
					Name: "shard-b",
				},
			}

		}))
		defer deleteFluxShardSetAndFinalize(t, k8sClient, reconciler, shardset)

		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Annotations = map[string]string{}
			d.ObjectMeta.Name = "kustomize-controller"
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		reconciler.Create(ctx, srcDeployment)

		// Reconcile
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
		test.AssertNoError(t, err)

		// Check fluxshardset
		updated := &templatesv1.FluxShardSet{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shardset), updated))
		assertDeploymentsExist(t, k8sClient, "default", "shard-a-kustomize-controller", "shard-b-kustomize-controller")

		// Removing shard
		shardset.Spec.Shards = shardset.Spec.Shards[:1]
		// Adding shard
		shardset.Spec.Shards = append(shardset.Spec.Shards, templatesv1.ShardSpec{
			Name: "shard-c",
		})
		reconcileAndAssertFinalizerExists(t, k8sClient, reconciler, shardset)

		assertDeploymentsExist(t, k8sClient, "default", "shard-a-kustomize-controller", "shard-c-kustomize-controller")
		assertDeploymentsDontExist(t, k8sClient, "default", "shard-b-kustomize-controller")

	})

	// t.Run("don't create deployments", func(t *testing.T) {
	// 	ctx := context.TODO()
	// 	// Create shard set and src deployment
	// 	shardset := createAndReconcileToFinalizedState(t, k8sClient, reconciler, makeTestFluxShardSet(t, func(shardset *templatesv1.FluxShardSet) {
	// 		shardset.Spec.Type = "kustomize"
	// 		shardset.Spec.Shards = []templatesv1.ShardSpec{
	// 			{
	// 				Name: "shard-1",
	// 			},
	// 		}

	// 	}))
	// 	defer deleteFluxShardSetAndFinalize(t, k8sClient, reconciler, shardset)

	// 	srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
	// 		d.Annotations = map[string]string{}
	// 		d.ObjectMeta.Name = "kustomize-controller"
	// 	})
	// 	reconciler.Create(ctx, srcDeployment)

	// 	// Reconcile
	// 	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
	// 	test.AssertNoError(t, err)
	// 	test.AssertErrorMatch(t, "Sharding label not found", err)

	// 	// Check fluxshardset
	// 	updated := &templatesv1.FluxShardSet{}
	// 	test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shardset), updated))
	// 	assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller", "shard-2-kustomize-controller")

	// 	reconcileAndAssertFinalizerExists(t, k8sClient, reconciler, shardset)
	// 	// Check deployment for shard-1 wasn't created
	// 	assertDeploymentsDontExist(t, k8sClient, "default", "shard-1-kustomize-controller")

	// })

}

func assertDeploymentsExist(t *testing.T, cl client.Client, ns string, want ...string) {
	t.Helper()
	d := &appsv1.DeploymentList{}
	test.AssertNoError(t, cl.List(context.TODO(), d, client.InNamespace(ns)))

	existingNames := func(l []appsv1.Deployment) []string {
		names := []string{}
		for _, v := range l {
			names = append(names, v.GetName())
		}
		sort.Strings(names)
		return names
	}(d.Items)

	sort.Strings(want)
	if diff := cmp.Diff(want, existingNames); len(diff) < 0 {
		t.Fatalf("got different names:\n%s", diff)
	}
}

func assertDeploymentsDontExist(t *testing.T, cl client.Client, ns string, deps ...string) {
	t.Helper()
	d := &appsv1.DeploymentList{}
	test.AssertNoError(t, cl.List(context.TODO(), d, client.InNamespace(ns)))

	existingNames := func(l []appsv1.Deployment) []string {
		names := []string{}
		for _, v := range l {
			names = append(names, v.GetName())
		}
		sort.Strings(names)
		return names
	}(d.Items)

	sort.Strings(deps)
	if diff := cmp.Diff(deps, existingNames); len(diff) < 0 {

		t.Fatalf("Found deployments that shouldn't be found:\n%s", diff)
	}
}

func assertFluxShardSetCondition(t *testing.T, shardset *templatesv1.FluxShardSet, condType, msg string) {
	t.Helper()
	cond := apimeta.FindStatusCondition(shardset.Status.Conditions, condType)
	if cond == nil {
		t.Fatalf("failed to find matching status condition for type %s in %#v", condType, shardset.Status.Conditions)
	}
	if cond.Message != msg {
		t.Fatalf("got %s, want %s", cond.Message, msg)
	}
}

// Create the provided FluxShardSet
func createAndReconcileToFinalizedState(t *testing.T, k8sClient client.Client, r *FluxShardSetReconciler, shardset *templatesv1.FluxShardSet) *templatesv1.FluxShardSet {
	test.AssertNoError(t, k8sClient.Create(context.TODO(), shardset))
	reconcileAndAssertFinalizerExists(t, k8sClient, r, shardset)

	return shardset
}

// this runs a single reconciliation and asserts that the set finalizer is
// applied
// This is needed because the reconciler returns after applying the finalizer to
// avoid race conditions.
func reconcileAndAssertFinalizerExists(t *testing.T, cl client.Client, reconciler *FluxShardSetReconciler, shardset *templatesv1.FluxShardSet) {
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
	test.AssertNoError(t, err)

	test.AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset))
	if !controllerutil.ContainsFinalizer(shardset, templatesv1.FluxShardSetFinalizer) {
		t.Fatal("FluxShardSet is missing the finalizer")
	}
}

func makeTestFluxShardSet(t *testing.T, opts ...func(*templatesv1.FluxShardSet)) *templatesv1.FluxShardSet {
	fluxshardset := &templatesv1.FluxShardSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-shard-set",
			Namespace: "default",
		},
		Spec: templatesv1.FluxShardSetSpec{
			Type:   "kustomize",
			Shards: []templatesv1.ShardSpec{},
		},
	}

	for _, o := range opts {
		o(fluxshardset)
	}

	return fluxshardset
}

func deleteFluxShardSetAndFinalize(t *testing.T, cl client.Client, reconciler *FluxShardSetReconciler, shardset *templatesv1.FluxShardSet) {
	t.Helper()
	ctx := context.TODO()
	if shardset.Spec.Suspend {
		shardset.Spec.Suspend = false
		test.AssertNoError(t, cl.Update(ctx, shardset))
	}

	test.AssertNoError(t, cl.Delete(ctx, shardset))

	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
	test.AssertNoError(t, err)

	if !apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset)) {
		t.Fatalf("failed to finalize: %s", err)
	}
	test.AssertNoError(t, client.IgnoreNotFound(cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset)))
}

func mustMarshalJSON(t *testing.T, r runtime.Object) []byte {
	b, err := json.Marshal(r)
	test.AssertNoError(t, err)

	return b
}

func nsn(namespace, name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
}
