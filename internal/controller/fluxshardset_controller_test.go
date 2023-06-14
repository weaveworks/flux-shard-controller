package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-cmp/cmp"
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
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		shardSet := newTestFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name:      srcDeployment.Name,
				Namespace: srcDeployment.Namespace,
			}
		})

		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		wantDeployment := test.MakeTestDeployment(nsn("default", "shard-1-kustomize-controller"), func(d *appsv1.Deployment) {
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
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(wantDeployment), wantDeployment))

		// Check inventory updated with fluxshardset and new deployment(want) and condition of number of resources created
		test.AssertInventoryHasItems(t, shardSet, want...)
		assertFluxShardSetCondition(t, shardSet, meta.ReadyCondition, "1 shard(s) created")

		// Check deployments existing include the new deployment
		assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller")
	})

	t.Run("reconciling creation of new deployment when it already exists", func(t *testing.T) {
		ctx := context.TODO()
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		shardSet := newTestFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-1",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name:      srcDeployment.Name,
				Namespace: srcDeployment.Namespace,
			}
		})

		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		test.AssertNoError(t, k8sClient.Create(ctx,
			test.MakeTestDeployment(nsn("default", "shard-1-kustomize-controller"), func(d *appsv1.Deployment) {
				d.ObjectMeta.Labels = map[string]string{
					"templates.weave.works/shard-set": "test-shard-set",
					"app.kubernetes.io/managed-by":    "flux-shard-controller",
				}
				d.Spec.Template.Spec.Containers[0].Args = []string{
					"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
				}
			})))

		// Reconcile
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardSet)})
		test.AssertErrorMatch(t, `failed to create Deployment: deployments.apps "shard-1-kustomize-controller" already exists`, err)

		// reload the shardset
		test.AssertNoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(shardSet), shardSet))

		if shardSet.Status.Inventory != nil {
			t.Errorf("expected Inventory to be nil, but got %v", shardSet.Status.Inventory)
		}
		assertFluxShardSetCondition(t, shardSet, meta.ReadyCondition,
			`failed to create Deployment: deployments.apps "shard-1-kustomize-controller" already exists`)
	})

	t.Run("Delete resources when removing shard from fluxshardset shards", func(t *testing.T) {
		ctx := context.TODO()
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		reconciler.Create(ctx, srcDeployment)
		defer reconciler.Delete(ctx, srcDeployment)

		// Create shard set and src deployment
		shardSet := newTestFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name:      srcDeployment.Name,
				Namespace: srcDeployment.Namespace,
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
		assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller", "shard-2-kustomize-controller")

		shard1Deploy := test.MakeTestDeployment(nsn("default", "shard-1-kustomize-controller"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Labels = map[string]string{
				"templates.weave.works/shard-set": "test-shard-set",
				"app.kubernetes.io/managed-by":    "flux-shard-controller",
			}
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
			}
		})

		shard2Deploy := test.MakeTestDeployment(nsn("default", "shard-2-kustomize-controller"), func(d *appsv1.Deployment) {
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
		shardSet.Spec.Shards = shardSet.Spec.Shards[:1]
		test.AssertNoError(t, k8sClient.Update(ctx, shardSet))

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		// Check deployment for shard-1 exists and deployment for shard-2 is deleted
		test.AssertInventoryHasItems(t, shardSet, shard1Deploy)
		assertDeploymentsExist(t, k8sClient, "default", "shard-1-kustomize-controller")
		assertDeploymentsDontExist(t, k8sClient, "default", "shard-2-kustomize-controller")
	})

	t.Run("Create new deployments with new shard names and delete old deployments after removing shard names", func(t *testing.T) {
		ctx := context.TODO()
		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
			d.ObjectMeta.Name = "kustomize-controller"
			d.Spec.Template.Spec.Containers[0].Args = []string{
				"--watch-label-selector=!sharding.fluxcd.io/key",
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, srcDeployment))
		defer k8sClient.Delete(ctx, srcDeployment)

		// Create shard set and src deployment
		shardSet := newTestFluxShardSet(func(set *templatesv1.FluxShardSet) {
			set.Spec.Shards = []templatesv1.ShardSpec{
				{
					Name: "shard-a",
				},
				{
					Name: "shard-b",
				},
			}
			set.Spec.SourceDeploymentRef = templatesv1.SourceDeploymentReference{
				Name:      srcDeployment.Name,
				Namespace: srcDeployment.Namespace,
			}
		})
		test.AssertNoError(t, k8sClient.Create(ctx, shardSet))
		defer deleteFluxShardSet(t, k8sClient, shardSet)

		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		assertDeploymentsExist(t, k8sClient, "default", "shard-a-kustomize-controller", "shard-b-kustomize-controller")

		// Removing shard
		shardSet.Spec.Shards = []templatesv1.ShardSpec{
			{
				Name: "shard-a",
			},
			{
				Name: "shard-c",
			},
		}
		reconcileAndReload(t, k8sClient, reconciler, shardSet)

		createDeployment := func(shardID string) *appsv1.Deployment {
			return test.MakeTestDeployment(nsn("default", shardID+"-kustomize-controller"), func(d *appsv1.Deployment) {
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
		assertDeploymentsExist(t, k8sClient, "default", "shard-a-kustomize-controller", "shard-c-kustomize-controller")
		assertDeploymentsDontExist(t, k8sClient, "default", "shard-b-kustomize-controller")
	})

	// 	t.Run("don't create deployments if srcdeployments not ignoring sharding", func(t *testing.T) {
	// 		ctx := context.TODO()
	// 		// Create shard set and src deployment
	// 		shardset := createAndReconcile(t, k8sClient, reconciler, newTestFluxShardSet(t, func(shardset *templatesv1.FluxShardSet) {
	// 			shardset.Spec.Type = "kustomize"
	// 			shardset.Spec.Shards = []templatesv1.ShardSpec{
	// 				{
	// 					Name: "shard-1",
	// 				},
	// 			}

	// 		}))
	// 		defer deleteFluxShardSetAndWaitForNotFound(t, k8sClient, reconciler, shardset)

	// 		srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
	// 			d.Annotations = map[string]string{}
	// 			d.ObjectMeta.Name = "kustomize-controller"
	// 		})
	// 		reconciler.Create(ctx, srcDeployment)

	// 		// Reconcile
	// 		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
	// 		// Check for error matching expected error from deploys.generateDeployments
	// 		test.AssertErrorMatch(t, "failed to generate deployments: deployment default/kustomize-controller is not configured to ignore sharding", err)
	// 	})
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

// this runs a single reconciliation
func reconcileAndReload(t *testing.T, cl client.Client, reconciler *FluxShardSetReconciler, shardset *templatesv1.FluxShardSet) {
	t.Helper()
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(shardset)})
	test.AssertNoError(t, err)

	test.AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset))
}

func newTestFluxShardSet(opts ...func(*templatesv1.FluxShardSet)) *templatesv1.FluxShardSet {
	fluxshardset := &templatesv1.FluxShardSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-shard-set",
			Namespace: "default",
		},
		Spec: templatesv1.FluxShardSetSpec{},
	}

	for _, o := range opts {
		o(fluxshardset)
	}

	return fluxshardset
}

func deleteFluxShardSet(t *testing.T, cl client.Client, shardset *templatesv1.FluxShardSet) {
	t.Helper()
	test.AssertNoError(t, cl.Delete(context.TODO(), shardset))
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
