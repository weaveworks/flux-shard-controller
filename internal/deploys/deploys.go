package deploys

import (
	"fmt"
	"strings"

	"github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ignoreShardsSelector    = "!sharding.fluxcd.io/key"
	ignoreShardsSelectorArg = "--watch-label-selector=" + ignoreShardsSelector
	shardsSelector          = "sharding.fluxcd.io/key"
)

// newDeploymentFromDeployment takes a Deployment loaded from the Cluster and
// clears out the Metadata fields that are needed in the cluster.
func newDeploymentFromDeployment(src appsv1.Deployment) *appsv1.Deployment {
	depl := src.DeepCopy()
	depl.CreationTimestamp = metav1.Time{}
	if depl.Annotations == nil {
		depl.Annotations = map[string]string{}
	}

	// This is really a work around for the test cases
	// But it doesn't cost anything to do it here.
	// https://github.com/kubernetes-sigs/controller-runtime/issues?q=is%3Aissue+typemeta+empty+is%3Aclosed+
	depl.TypeMeta = metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	}
	delete(depl.Annotations, "deployment.kubernetes.io/revision")
	depl.ObjectMeta.Name = ""
	depl.Generation = 0
	depl.ResourceVersion = ""
	depl.UID = ""
	depl.Status = appsv1.DeploymentStatus{}

	return depl
}

// updateNewDeployment updates the deployment with sharding related fields such as name and required labels
func updateNewDeployment(depl *appsv1.Deployment, shardSetName, shardName, newDeploymentName string) error {
	// Add sharding labels
	if depl.ObjectMeta.Labels == nil {
		depl.ObjectMeta.Labels = map[string]string{}
	}

	shardLabels := map[string]string{
		"app.kubernetes.io/managed-by":    "flux-shard-controller",
		"templates.weave.works/shard-set": shardSetName,
		"templates.weave.works/shard":     shardName,
		"sharding.fluxcd.io/role":         "shard",
	}

	depl.ObjectMeta.Labels = merge(
		shardLabels,
		depl.ObjectMeta.Labels,
	)
	// generate selector args string
	selectorArgs, err := generateSelectorStr("--watch-label-selector", shardsSelector, metav1.LabelSelectorOpIn, []string{shardName})
	if err != nil {
		return err
	}

	for _, container := range depl.Spec.Template.Spec.Containers {
		if container.Args == nil {
			container.Args = []string{}
		}
		if container.Name == "manager" {
			replaceArg(container.Args, ignoreShardsSelectorArg, selectorArgs)
		}

	}

	// Update deployment name
	depl.ObjectMeta.Name = newDeploymentName

	// This makes the selector and template labels match.
	depl.Spec.Selector.MatchLabels = merge(
		shardLabels,
		depl.Spec.Selector.MatchLabels,
	)

	depl.Spec.Template.ObjectMeta.Labels = merge(
		shardLabels,
		depl.Spec.Template.ObjectMeta.Labels,
	)

	return nil
}

// return a copy off the "dest" map, with the elements of the "src" map applied
// over the top.
func merge[K comparable, V any](src, dest map[K]V) map[K]V {
	merged := map[K]V{}
	for k, v := range dest {
		merged[k] = v
	}
	for k, v := range src {
		merged[k] = v
	}

	return merged
}

// GenerateDeployments creates list of new deployments to process the set of
// shards declared in the ShardSet.
func GenerateDeployments(fluxShardSet *v1alpha1.FluxShardSet, src *appsv1.Deployment) ([]*appsv1.Deployment, error) {
	if !deploymentIgnoresShardLabels(src) {
		return nil, fmt.Errorf("deployment %s is not configured to ignore sharding", client.ObjectKeyFromObject(src))
	}
	generatedDeployments := []*appsv1.Deployment{}
	for _, shard := range fluxShardSet.Spec.Shards {
		deployment := newDeploymentFromDeployment(*src)
		newDeploymentName := fmt.Sprintf("%s-%s", src.ObjectMeta.Name, shard.Name)
		err := updateNewDeployment(deployment, fluxShardSet.Name, shard.Name, newDeploymentName)
		if err != nil {
			return nil, err
		}

		generatedDeployments = append(generatedDeployments, deployment)
	}

	return generatedDeployments, nil
}

func deploymentIgnoresShardLabels(deploy *appsv1.Deployment) bool {
	for i := range deploy.Spec.Template.Spec.Containers {
		container := deploy.Spec.Template.Spec.Containers[i]
		for _, arg := range container.Args {
			if arg == ignoreShardsSelectorArg {
				return true
			}
		}
	}

	return false
}

func replaceArg(args []string, ignoreShardsSelectorArgs string, newArg string) {
	for i := range args {
		if strings.HasPrefix(args[i], ignoreShardsSelectorArgs) {
			args[i] = newArg
		}
	}
}

func generateSelectorStr(key, shardSelector string, operator metav1.LabelSelectorOperator, values []string) (string, error) {
	selector := &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      shardSelector,
				Operator: operator,
				Values:   values,
			},
		},
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return "", fmt.Errorf("failed to generate label selector: %v", err)
	}

	return fmt.Sprintf("%s=%s", key, labelSelector), nil
}
