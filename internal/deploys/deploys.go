package deploys

import (
	"github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewDeploymentFromDeployment takes a Deployment loaded from the Cluster and
// clears out the Metadata fields that are needed in the cluster.
func NewDeploymentFromDeployment(src appsv1.Deployment) *appsv1.Deployment {
	src.CreationTimestamp = metav1.Time{}
	if len(src.Annotations) > 1 {
		delete(src.Annotations, "deployment.kubernetes.io/revision")
	} else {
		src.Annotations = nil
	}
	src.Generation = 0
	src.ResourceVersion = ""
	src.UID = ""
	src.Status = appsv1.DeploymentStatus{}
	src.Labels["app.kubernetes.io/managed-by"] = "flux-shard-controller"

	return &src
}

// GetDeploymentsMatchingFluxShardSet returns a list of deployments that match the shards in fluxShardSet given
func GetDeploymentsMatchingFluxShardSet(fluxShardSet *v1alpha1.FluxShardSet, allDeployments []*appsv1.Deployment) []*appsv1.Deployment {
	matchingDeployments := []*appsv1.Deployment{}

	for _, d := range allDeployments {
		if DeploymentMatchesShard(fluxShardSet, *d) {
			matchingDeployments = append(matchingDeployments, d)
		}
	}

	return matchingDeployments
}

// // Given a fluxShardSet, return a list of names/labels to match with deployments
// func ShardNamesFromFluxShardSets(fluxShardSet *v1alpha1.FluxShardSet) ([]string, error) {
// 	shards := []string{}
// 	for _, shard := range fluxShardSet.Spec.Shards {
// 		shards = append(shards, shard.Name)
// 	}
// 	return shards, nil

// }

// DeploymentMatchesShard returns true if the deployment matches a shard in the fluxShardSet
// matching is done by comparing the shards list and the deployment name
func DeploymentMatchesShard(fluxShardSet *v1alpha1.FluxShardSet, deployment appsv1.Deployment) bool {
	for _, shard := range fluxShardSet.Spec.Shards {
		if shard.Name == deployment.Name {
			return true
		}
	}
	return false
}

// GenerateDeployments creates list of new deployments from the given deployments that match the filtering in the fluxShardSet
func GenerateDeployments(fluxShardSet *v1alpha1.FluxShardSet, deployments []*appsv1.Deployment) []*appsv1.Deployment {
	matchingDeps := GetDeploymentsMatchingFluxShardSet(fluxShardSet, deployments)

	// Generate new deployments
	newDeployments := []*appsv1.Deployment{}
	for _, existingDeployment := range matchingDeps {
		newDeployment := NewDeploymentFromDeployment(*existingDeployment)
		newDeployments = append(newDeployments, newDeployment)
	}
	return newDeployments

}
