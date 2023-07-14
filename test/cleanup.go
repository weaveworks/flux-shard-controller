package test

import (
	"context"
	"fmt"
	"testing"

	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteFluxShardSet will remove all the resources in the inventory and remove
// the resource.
// This is only needed in tests because Owned resources are not automatically
// cleaned up.
func DeleteFluxShardSet(t *testing.T, cl client.Client, shardset *templatesv1.FluxShardSet) {
	ctx := context.TODO()
	t.Helper()

	AssertNoError(t, cl.Get(ctx, client.ObjectKeyFromObject(shardset), shardset))

	if shardset.Status.Inventory != nil {
		for _, v := range shardset.Status.Inventory.Entries {
			d, err := unstructuredFromResourceRef(v)
			AssertNoError(t, err)
			AssertNoError(t, cl.Delete(ctx, d))
		}
	}

	AssertNoError(t, cl.Delete(ctx, shardset))
}

func unstructuredFromResourceRef(ref templatesv1.ResourceRef) (*unstructured.Unstructured, error) {
	objMeta, err := object.ParseObjMetadata(ref.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse object ID %s: %w", ref.ID, err)
	}
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(objMeta.GroupKind.WithVersion(ref.Version))
	u.SetName(objMeta.Name)
	u.SetNamespace(objMeta.Namespace)

	return &u, nil
}
