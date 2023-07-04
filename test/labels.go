package test

// ShardLabels creates a map with the set of labels that are applied to
// resources.
//
// If we change the core labels, this needs to change.
func ShardLabels(shardName string, additional ...map[string]string) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/managed-by":    "flux-shard-controller",
		"templates.weave.works/shard":     shardName,
		"templates.weave.works/shard-set": "test-shard-set",
		"sharding.fluxcd.io/role":         "shard",
	}

	for _, m := range additional {
		for k, v := range m {
			labels[k] = v
		}
	}

	return labels
}
