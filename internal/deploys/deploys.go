package deploys

import (
	"fmt"
	"strings"

	"github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ignoreShardsSelector = "!sharding.fluxcd.io/key"
const shardsSelector = "sharding.fluxcd.io/key"

// NewDeploymentFromDeployment takes a Deployment loaded from the Cluster and
// clears out the Metadata fields that are needed in the cluster.
func NewDeploymentFromDeployment(src appsv1.Deployment) *appsv1.Deployment {
	depl := src.DeepCopy()
	depl.CreationTimestamp = metav1.Time{}
	if len(depl.Annotations) > 1 {
		delete(depl.Annotations, "deployment.kubernetes.io/revision")
	} else {
		depl.Annotations = nil
	}
	depl.Generation = 0
	depl.ResourceVersion = ""
	depl.UID = ""
	depl.Status = appsv1.DeploymentStatus{}

	return depl
}

// UpdateNewDeployment updates the deployment with sharding related fields such as name and required labels
func UpdateNewDeployment(depl *appsv1.Deployment, shardsetName string, shardName string) {
	// Add sharding labels
	depl.Labels = map[string]string{}
	depl.Labels["app.kubernetes.io/managed-by"] = "flux-shard-controller"
	depl.ObjectMeta.Labels["templates.weave.works/shard-set"] = shardsetName
	selectorArgs := fmt.Sprintf("--watch-label-selector=%s in (%s)", shardsSelector, shardName)
	depl.Spec.Template.Spec.Containers[0].Args = []string{selectorArgs}

	// Update deplyment name
	depl.ObjectMeta.Name = fmt.Sprintf("%s-%s", shardName, depl.ObjectMeta.Name)
}

// GenerateDeployments creates list of new deployments to process the set of
// shards declared in the ShardSet.
func GenerateDeployments(fluxShardSet *v1alpha1.FluxShardSet, src *appsv1.Deployment) ([]*appsv1.Deployment, error) {
	if !deploymentIgnoresShardLabels(src) {
		return nil, fmt.Errorf("deployment %s is not configured to ignore sharding", client.ObjectKeyFromObject(src))
	}
	generatedDeployments := []*appsv1.Deployment{}
	for _, shard := range fluxShardSet.Spec.Shards {
		deployment := NewDeploymentFromDeployment(*src)
		UpdateNewDeployment(deployment, fluxShardSet.Name, shard.Name)
		generatedDeployments = append(generatedDeployments, deployment)
	}

	if len(generatedDeployments) == 0 {
		return nil, nil
	}
	return generatedDeployments, nil
}

func deploymentIgnoresShardLabels(deploy *appsv1.Deployment) bool {
	wantArg := fmt.Sprintf("--watch-label-selector=%s", ignoreShardsSelector)
	for i := range deploy.Spec.Template.Spec.Containers {
		container := deploy.Spec.Template.Spec.Containers[i]
		for _, arg := range container.Args {
			if strings.HasPrefix(arg, wantArg) {
				return true
			}
		}
	}

	return false
}
