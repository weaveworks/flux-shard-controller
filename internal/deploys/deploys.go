package deploys

import (
	"fmt"

	"github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ignoreShardsSelector = "!sharding.fluxcd.io/key"

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

// GenerateDeployments creates list of new deployments to process the set of
// shards declared in the ShardSet.
func GenerateDeployments(fluxShardSet *v1alpha1.FluxShardSet, src *appsv1.Deployment) ([]*appsv1.Deployment, error) {
	if !deploymentIgnoresShardLabels(src) {
		return nil, fmt.Errorf("deployment %s is not configured to ignore sharding", client.ObjectKeyFromObject(src))
	}

	return nil, nil
}

func deploymentIgnoresShardLabels(deploy *appsv1.Deployment) bool {
	wantArg := fmt.Sprintf("--watch-label-selector=%s", ignoreShardsSelector)
	for i := range deploy.Spec.Template.Spec.Containers {
		container := deploy.Spec.Template.Spec.Containers[i]
		for _, arg := range container.Args {
			if arg == wantArg {
				return true
			}
		}
	}

	return false
}
