package test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
)

// NewFluxShardSet creates and returns a new ShardSet with the option to
// configure it.
func NewFluxShardSet(opts ...func(*templatesv1.FluxShardSet)) *templatesv1.FluxShardSet {
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
