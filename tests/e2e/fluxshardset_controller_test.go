package tests

import (
	"context"
	"regexp"
	"sort"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"github.com/weaveworks/flux-shard-controller/test"
)

func TestCreatingDeployments(t *testing.T) {
	ctx := context.TODO()
	// Create a new GitopsCluster object and ensure it is created
	srcDeployment := test.MakeTestDeployment(nsn("default", "kustomize-controller"), func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Args = []string{
			"--watch-label-selector=!sharding.fluxcd.io/key",
		}
	})
	test.AssertNoError(t, testEnv.Create(ctx, srcDeployment))
	defer testEnv.Delete(ctx, srcDeployment)

	shardSet := test.NewFluxShardSet(func(set *templatesv1.FluxShardSet) {
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

	test.AssertNoError(t, testEnv.Create(ctx, shardSet))
	defer testEnv.Delete(ctx, shardSet)

	waitForFluxShardSetCondition(t, testEnv, shardSet, `1 shard\(s\) created`)
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

		want := generateResourceInventory(objs)

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
func generateResourceInventory(objs []runtime.Object) *templatesv1.ResourceInventory {
	entries := []templatesv1.ResourceRef{}
	for _, obj := range objs {
		ref, err := templatesv1.ResourceRefFromObject(obj)
		if err != nil {
			panic(err)
		}
		entries = append(entries, ref)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return &templatesv1.ResourceInventory{Entries: entries}
}

// func deleteFluxShardSetAndWaitForNotFound(t *testing.T, cl client.Client, set *templatesv1.FluxShardSet) {
// 	t.Helper()
// 	deleteObject(t, cl, set)

// 	g := gomega.NewWithT(t)
// 	g.Eventually(func() bool {
// 		updated := &templatesv1.FluxShardSet{}
// 		return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(set), updated))
// 	}, timeout).Should(gomega.BeTrue())
// }

func nsn(namespace, name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
}
