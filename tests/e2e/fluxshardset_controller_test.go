package tests

import (
	"context"
	"regexp"
	"sort"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"

	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"github.com/weaveworks/flux-shard-controller/test"
)

var ignoreObjectMeta = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "UID", "OwnerReferences", "ResourceVersion", "Generation", "CreationTimestamp", "ManagedFields", "Finalizers")

func TestCreatingDeployments(t *testing.T) {
	ctx := context.TODO()
	srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Args = []string{
			"--watch-label-selector=!sharding.fluxcd.io/key",
		}
	})
	test.AssertNoError(t, testEnv.Create(ctx, srcDeployment))
	defer func() {
		test.AssertNoError(t, testEnv.Get(ctx, client.ObjectKeyFromObject(srcDeployment), srcDeployment))
		deleteObject(t, testEnv, srcDeployment)
	}()

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

	test.AssertNoError(t, testEnv.Create(ctx, shardSet))
	defer deleteShardSetAndWaitForNotFound(t, testEnv, shardSet)

	waitForFluxShardSetCondition(t, testEnv, shardSet, `1 shard\(s\) created`)
	want := test.MakeTestDeployment(nsn(srcDeployment.GetNamespace(), "kustomize-controller-shard-1"), func(d *appsv1.Deployment) {
		d.ObjectMeta.Labels = test.ShardLabels("shard-1")
		d.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: test.ShardLabels("shard-1", map[string]string{
				"app": "kustomize-controller",
			}),
		}
		d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-1", map[string]string{
			"app": "kustomize-controller",
		})
		d.Spec.Template.Spec.Containers[0].Args = []string{
			"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
		}
	})

	var created appsv1.Deployment
	test.AssertNoError(t, testEnv.Get(ctx, client.ObjectKeyFromObject(want), &created))
	if diff := cmp.Diff(want, &created, ignoreObjectMeta); diff != "" {
		t.Fatalf("failed to create Deployment:\n%s", diff)
	}
}

func TestUpdatingDeployments(t *testing.T) {
	ctx := context.TODO()
	srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Args = []string{
			"--watch-label-selector=!sharding.fluxcd.io/key",
		}
		d.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.0"
	})
	test.AssertNoError(t, testEnv.Create(ctx, srcDeployment))
	defer func() {
		test.AssertNoError(t, testEnv.Get(ctx, client.ObjectKeyFromObject(srcDeployment), srcDeployment))
		deleteObject(t, testEnv, srcDeployment)
	}()

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

	test.AssertNoError(t, testEnv.Create(ctx, shardSet))
	defer deleteShardSetAndWaitForNotFound(t, testEnv, shardSet)
	waitForFluxShardSetCondition(t, testEnv, shardSet, `1 shard\(s\) created`)
	waitForFluxShardSetInventory(t, testEnv, shardSet, test.MakeTestDeployment(nsn("default", "kustomize-controller-shard-1")))

	test.AssertNoError(t, testEnv.Get(ctx, client.ObjectKeyFromObject(srcDeployment), srcDeployment))
	srcDeployment.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.2"
	test.AssertNoError(t, testEnv.Update(ctx, srcDeployment))

	test.AssertNoError(t, testEnv.Get(ctx, client.ObjectKeyFromObject(srcDeployment), srcDeployment))

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
		d.Spec.Template.Spec.Containers[0].Image = "ghcr.io/fluxcd/kustomize-controller:v0.35.2"
	})

	g := gomega.NewWithT(t)
	g.Eventually(func() string {
		createdDeploy := &appsv1.Deployment{}
		if err := testEnv.Get(context.TODO(), client.ObjectKeyFromObject(shard1Deploy), createdDeploy); err != nil {
			return err.Error()
		}

		return cmp.Diff(shard1Deploy, createdDeploy, ignoreObjectMeta)
	}, timeout).Should(gomega.BeEmpty())
}

func waitForFluxShardSetInventory(t *testing.T, k8sClient client.Client, set *templatesv1.FluxShardSet, objs ...runtime.Object) {
	t.Helper()
	g := gomega.NewWithT(t)

	g.Eventually(func() bool {
		updated := &templatesv1.FluxShardSet{}
		if err := k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(set), updated); err != nil {
			return false
		}

		if updated.Status.Inventory == nil {
			return false
		}

		if l := len(updated.Status.Inventory.Entries); l != len(objs) {
			t.Errorf("expected %d items, got %v", len(objs), l)
		}

		want := generateResourceInventory(t, objs)

		return cmp.Diff(want, updated.Status.Inventory) == ""
	}, timeout).Should(gomega.BeTrue())
}

func waitForFluxShardSetCondition(t *testing.T, k8sClient client.Client, set *templatesv1.FluxShardSet, message string) {
	t.Helper()
	g := gomega.NewWithT(t)
	g.Eventually(func() bool {
		updated := &templatesv1.FluxShardSet{}
		if err := k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(set), updated); err != nil {
			return false
		}
		cond := apimeta.FindStatusCondition(updated.Status.Conditions, meta.ReadyCondition)
		if cond == nil {
			return false
		}

		match, err := regexp.MatchString(message, cond.Message)
		if err != nil {
			t.Fatal(err)
		}

		if !match {
			t.Logf("failed to match %q to %q", message, cond.Message)
		}
		return match
	}, timeout).Should(gomega.BeTrue())
}

// generateResourceInventory generates a ResourceInventory object from a list of runtime objects.
func generateResourceInventory(t *testing.T, objs []runtime.Object) *templatesv1.ResourceInventory {
	entries := []templatesv1.ResourceRef{}
	for _, obj := range objs {
		ref, err := templatesv1.ResourceRefFromObject(obj)
		test.AssertNoError(t, err)
		entries = append(entries, ref)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return &templatesv1.ResourceInventory{Entries: entries}
}

func deleteObject(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()
	test.AssertNoError(t, cl.Delete(context.TODO(), obj))
}

// Owned resources are not automatically deleted in the testenv setup.
// This cleans the resources from the inventory, and then removes the Shard Set
// and waits for it to be gone.
func deleteShardSetAndWaitForNotFound(t *testing.T, cl client.Client, set *templatesv1.FluxShardSet) {
	t.Helper()
	ctx := context.TODO()
	test.AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(set), set))

	if set.Status.Inventory != nil {
		for _, v := range set.Status.Inventory.Entries {
			t.Logf("deleting %s", v.ID)
			objMeta, err := object.ParseObjMetadata(v.ID)
			if err != nil {
				t.Logf("failed to delete resource: %s", v.ID)
				t.Errorf("failed to delete resource ref %s when cleaning up", v.ID)
				continue
			}
			var deploy appsv1.Deployment
			test.AssertNoError(t, cl.Get(ctx, client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}, &deploy))

			if err := cl.Delete(ctx, &deploy); err != nil {
				t.Errorf("failed to delete deployment %+v when cleaning up", deploy.ObjectMeta)
			}
		}
	}

	deleteObject(t, cl, set)

	g := gomega.NewWithT(t)
	g.Eventually(func() bool {
		updated := &templatesv1.FluxShardSet{}
		return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(set), updated))
	}, timeout).Should(gomega.BeTrue())

	var deploymentList appsv1.DeploymentList
	test.AssertNoError(t, cl.List(ctx, &deploymentList, client.InNamespace("default")))

	for _, item := range deploymentList.Items {
		t.Logf("after deletion found: %+v", item.ObjectMeta)
	}

	if len(deploymentList.Items) > 1 {
		t.Fatalf("got %v deployments, want 1", len(deploymentList.Items))
	}
}

func nsn(namespace, name string) client.ObjectKey {
	return types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
}
