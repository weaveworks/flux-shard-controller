package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"github.com/weaveworks/flux-shard-controller/test"
)

var ignoreObjectMeta = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "UID", "OwnerReferences", "ResourceVersion", "Generation", "CreationTimestamp", "ManagedFields", "Finalizers")

// This is because the test env doesn't get the TypeMeta data back when
// querying.
var ignoreTypeMeta = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")

func TestReconciliation(t *testing.T) {
	scheme := runtime.NewScheme()
	test.AssertNoError(t, clientgoscheme.AddToScheme(scheme))
	test.AssertNoError(t, templatesv1.AddToScheme(scheme))

	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			"testdata/crds",
		},
		Scheme: scheme,
	}
	testEnv.ControlPlane.GetAPIServer().Configure().Append("--authorization-mode=RBAC")

	cfg, err := testEnv.Start()
	test.AssertNoError(t, err)
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop the test environment: %s", err)
		}
	}()

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
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer deleteObject(t, k8sClient, srcDeployment)

		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
		})

		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		wantDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-1"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = map[string]string{
				"templates.weave.works/shard-set": "test-shard-set",
				"app.kubernetes.io/managed-by":    "flux-shard-controller",
			}
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
		})
		want := []runtime.Object{
			wantDeployment,
		}

		// Check inventory updated with fluxshardset and new deployment(want) and condition of number of resources created
		test.AssertInventoryHasItems(t, shardSet, want...)
		assertFluxShardSetCondition(t, shardSet, meta.ReadyCondition, "1 shard(s) created")

		// Check deployments existing include the new deployment
		assertDeploymentsExist(t, k8sClient, "default", "kustomize-controller", "kustomize-controller-shard-1")
	})

	t.Run("reconciling creation of new deployment when it already exists", func(t *testing.T) {
		ctx := context.TODO()

		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer deleteObject(t, k8sClient, srcDeployment)

		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
		})

		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		shard1 := test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-1"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = map[string]string{
				"templates.weave.works/shard-set": "test-shard-set",
				"app.kubernetes.io/managed-by":    "flux-shard-controller",
			}
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, shard1))
		defer deleteObject(t, k8sClient, shard1)

		// Reconcile
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardSet)})
		test.AssertErrorMatch(t, `failed to create Deployment: deployments.apps "kustomize-controller-shard-1" already exists`, err)

		// reload the shardset
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shardSet), shardSet))

		if shardSet.Status.Inventory != nil {
			t.Errorf("expected Inventory to be nil, but got %v", shardSet.Status.Inventory)
		}
		assertFluxShardSetCondition(t, shardSet, meta.ReadyCondition,
			`failed to create Deployment: deployments.apps "kustomize-controller-shard-1" already exists`)
	})

	t.Run("Delete resources when removing shard from fluxshardset shards", func(t *testing.T) {
		ctx := context.TODO()
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer deleteObject(t, k8sClient, srcDeployment)

		// Create shard set and src deployment
		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
				{
					Name: "shard-2",
				},
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		// Check fluxshardset
		assertDeploymentsExist(t, k8sClient, "default", "kustomize-controller", "kustomize-controller-shard-1", "kustomize-controller-shard-2")

		shard1Deploy := test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-1"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = map[string]string{
				"templates.weave.works/shard-set": "test-shard-set",
				"app.kubernetes.io/managed-by":    "flux-shard-controller",
			}
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
		})

		shard2Deploy := test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-2"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = map[string]string{
				"templates.weave.works/shard-set": "test-shard-set",
				"app.kubernetes.io/managed-by":    "flux-shard-controller",
			}
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-2)",
			}
		})
		test.AssertInventoryHasItems(t, shardSet, shard1Deploy, shard2Deploy)

		// Update shard set by removing shard-2
		shardSet.Spec.Shards = []templatesv1.ShardSpec{
			{
				Name: "shard-1",
			},
		}
		test.AssertNoError(t, k8sClient.Update(ctx, shardSet))
		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		// Check deployment for shard-1 exists and deployment for shard-2 is deleted
		test.AssertInventoryHasItems(t, shardSet, shard1Deploy)
		assertDeploymentsExist(t, k8sClient, "default", "kustomize-controller", "kustomize-controller-shard-1")
		assertDeploymentsDontExist(t, k8sClient, "default", "shard-2-kustomize-controller")
	})

	t.Run("Create new deployments with new shard names and delete old deployments after removing shard names", func(t *testing.T) {
		ctx := context.TODO()
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		// Create shard set and src deployment
		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-a",
				},
				{
					Name: "shard-b",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		assertDeploymentsExist(t, k8sClient, "default", "kustomize-controller", "kustomize-controller-shard-a", "kustomize-controller-shard-b")

		// Removing shard
		shardSet.Spec.Shards = []templatesv1.ShardSpec{
			{
				Name: "shard-a",
			},
			{
				Name: "shard-c",
			},
		}
		test.AssertNoError(t, k8sClient.Update(ctx, shardSet))
		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		createDeployment := func(shardID string) *appsv1.Deployment {
			return test.MakeTestDeployment(nsn("default", "kustomize-controller-"+shardID), func(d *appsv1.Deployment) {
				d.ObjectMeta.Labels = map[string]string{
					"templates.weave.works/shard-set": "test-shard-set",
					"app.kubernetes.io/managed-by":    "flux-shard-controller",
				}
				d.Spec.Template.Spec.Containers[0].Args = []string{
					fmt.Sprintf("--watch-label-selector=sharding.fluxcd.io/key in (%s)", shardID),
				}
			})
		}

		test.AssertInventoryHasItems(t, shardSet, createDeployment("shard-a"), createDeployment("shard-c"))
		assertDeploymentsExist(t, k8sClient, "default", "kustomize-controller", "kustomize-controller-shard-a", "kustomize-controller-shard-c")
		assertDeploymentsDontExist(t, k8sClient, "default", "shard-b-kustomize-controller")
	})

	t.Run("don't create deployments if src deployment not ignoring sharding", func(t *testing.T) {
		ctx := context.TODO()
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Annotations = map[string]string{}
			d.ObjectMeta.Name = "kustomize-controller"
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		// Create shard set and src deployment
		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		expectedErrMsg := "failed to generate deployments: deployment default/kustomize-controller is not configured to ignore sharding"
		reconcileWithErrorAndReload(t, k8sClient, reconciler, shardSet, expectedErrMsg)

		assertFluxShardSetCondition(t, shardSet, meta.ReadyCondition, expectedErrMsg)
	})

	t.Run("Update generated deployments when src deployment updated existing annotations", func(t *testing.T) {
		ctx := context.TODO()

		// Create shard set and src deployment
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
			d.Annotations = map[string]string{
				"deployment.kubernetes.io/revision": "1",
			}

		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
		})

		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		shard1Deploy := test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-1"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = test.ShardLabels("shard-1")
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
			d.Spec.Selector.MatchLabels = test.ShardLabels("shard-1", map[string]string{
				"app": srcDeployment.Name,
			})
			d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-1", map[string]string{
				"app": srcDeployment.Name,
			})
			d.Spec.Template.Spec.ServiceAccountName = srcDeployment.Name

		})
		genDeployment := &appsv1.Deployment{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shard1Deploy), genDeployment))
		if diff := cmp.Diff(genDeployment, shard1Deploy, ignoreObjectMeta, ignoreTypeMeta); diff != "" {
			t.Fatalf("Generated deployment does not match expected, diff: %s", diff)
		}

		// Update src deployment
		srcDeployment.Annotations = map[string]string{
			"deployment.kubernetes.io/revision": "1",
			"test-annot":                        "test",
		}
		test.AssertNoError(t, k8sClient.Update(ctx, srcDeployment))

		shard1Deploy.Annotations = map[string]string{
			"test-annot": "test",
		}
		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		updatedGenDepl := &appsv1.Deployment{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shard1Deploy), updatedGenDepl))
		if diff := cmp.Diff(updatedGenDepl, shard1Deploy, ignoreObjectMeta, ignoreTypeMeta); diff != "" {
			t.Fatalf("generated deployments don't match expected, diff: %s", diff)
		}

		test.AssertInventoryHasItems(t, shardSet, shard1Deploy)

	})

	t.Run("Update generated deployments when src deployment updated existing container image", func(t *testing.T) {
		ctx := context.TODO()
		// Create shard set and src deployment
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
			d.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.1"

		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name: srcDeployment.Name,
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)
		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		shard1Deploy := test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-1"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = test.ShardLabels("shard-1")
			d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-1")
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
			d.Spec.Selector.MatchLabels = test.ShardLabels("shard-1", map[string]string{
				"app": srcDeployment.Name,
			})
			d.Spec.Template.ObjectMeta.Labels["app"] = srcDeployment.Name
			d.Spec.Template.Spec.ServiceAccountName = srcDeployment.Name
			srcDeployment.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.1"
		})

		genDeployment := &appsv1.Deployment{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shard1Deploy), genDeployment))
		if diff := cmp.Diff(genDeployment, shard1Deploy, ignoreObjectMeta, ignoreTypeMeta); diff != "" {
			t.Fatalf("Generated deployment does not match expected, diff: %s", diff)
		}

		// Update src deployment container image version and shard1Deploy
		srcDeployment.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.2"
		test.AssertNoError(t, k8sClient.Update(ctx, srcDeployment))

		shard1Deploy.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.2"
		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		updatedGenDepl := &appsv1.Deployment{}
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shard1Deploy), updatedGenDepl))
		if diff := cmp.Diff(updatedGenDepl, shard1Deploy, ignoreObjectMeta, ignoreTypeMeta); diff != "" {
			t.Fatalf("generated deployments don't match expected, diff: %s", diff)
		}

		test.AssertInventoryHasItems(t, shardSet, shard1Deploy)
	})
}

func assertDeploymentsExist(t *testing.T, cl client.Client, ns string, want ...string) {
	t.Helper()
	d := &appsv1.DeploymentList{}
	test.AssertNoError(t, cl.List(context.TODO(), d, client.InNamespace(ns)))

	existingDeps := []string{}
	for _, dep := range d.Items {
		existingDeps = append(existingDeps, dep.Name)
	}
	if diff := cmp.Diff(want, existingDeps); diff != "" {
		t.Fatalf("didn't find deployments, got different names: \n%s", diff)
	}
}

func assertDeploymentsDontExist(t *testing.T, cl client.Client, ns string, deps ...string) {
	t.Helper()
	d := &appsv1.DeploymentList{}
	test.AssertNoError(t, cl.List(context.TODO(), d, client.InNamespace(ns)))

	existingDeps := []string{}
	for _, dep := range d.Items {
		existingDeps = append(existingDeps, dep.Name)
	}

	matches := []string{}
	for _, dep := range deps {
		for _, existingDep := range existingDeps {
			if dep == existingDep {
				matches = append(matches, dep)
			}
		}
	}
	if len(matches) > 0 {
		cmp.Diff(matches, []string{})
		t.Fatalf("found deployments that shouldn't be found:\n%s", cmp.Diff(matches, []string{}))
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

// this runs a single reconciliation
func reconcileAndReload(t *testing.T, cl client.Client, reconciler *FluxShardSetReconciler, shardset *templatesv1.FluxShardSet) {
	t.Helper()
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
	test.AssertNoError(t, err)

	test.AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset))
}

func reconcileWithErrorAndReload(t *testing.T, cl client.Client, reconciler *FluxShardSetReconciler, shardSet *templatesv1.FluxShardSet, expectedErrMsg string) {
	t.Helper()
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardSet)})
	// Check for error matching expected error
	test.AssertErrorMatch(t, expectedErrMsg, err)

	// reload
	test.AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(shardSet), shardSet))
}

func deleteFluxShardSet(t *testing.T, cl client.Client, shardset *templatesv1.FluxShardSet) {
	ctx := context.TODO()
	t.Helper()

	test.AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset))

	if shardset.Status.Inventory != nil {
		for _, v := range shardset.Status.Inventory.Entries {
			d, err := deploymentFromResourceRef(v)
			test.AssertNoError(t, err)
			test.AssertNoError(t, cl.Delete(ctx, d))
		}
	}

	test.AssertNoError(t, cl.Delete(ctx, shardset))
}

func nsn(namespace, name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
}

func deleteObject(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()
	test.AssertNoError(t, cl.Delete(context.TODO(), obj))
}
