package deploys

import (
	"fmt"
	"strings"

	"github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ignoreShardsSelector    = "!sharding.fluxcd.io/key"
	ignoreShardsSelectorArg = "--watch-label-selector=" + ignoreShardsSelector
	storageAdvAddressArg    = "--storage-adv-addr="
	shardsSelector          = "sharding.fluxcd.io/key"
)

// GenerateDeployments creates list of new deployments to process the set of
// shards declared in the ShardSet.
func GenerateDeployments(fluxShardSet *v1alpha1.FluxShardSet, srcDeploy *appsv1.Deployment, srcSvc *corev1.Service) ([]client.Object, error) {
	if !deploymentIgnoresShardLabels(srcDeploy) {
		return nil, fmt.Errorf("deployment %s is not configured to ignore sharding", client.ObjectKeyFromObject(srcDeploy))
	}
	shardResources := []client.Object{}
	for _, shard := range fluxShardSet.Spec.Shards {
		shardLabels := map[string]string{
			"app.kubernetes.io/managed-by":    "flux-shard-controller",
			"templates.weave.works/shard-set": fluxShardSet.Name,
			"templates.weave.works/shard":     shard.Name,
			"sharding.fluxcd.io/role":         "shard",
		}

		newDeployment := newDeploymentFromDeployment(*srcDeploy)
		newDeploymentName := fmt.Sprintf("%s-%s", srcDeploy.ObjectMeta.Name, shard.Name)
		err := updateNewDeployment(newDeployment, fluxShardSet.Name, shard.Name, srcDeploy.ObjectMeta.Name, newDeploymentName, shardLabels)
		if err != nil {
			return nil, err
		}
		shardResources = append(shardResources, newDeployment)

		if srcSvc != nil {
			newSvc := newServiceFromService(*srcSvc, newDeploymentName, shardLabels, newDeployment.Spec.Template.ObjectMeta.Labels)
			shardResources = append(shardResources, newSvc)
		}
	}

	return shardResources, nil
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

func replaceArg(args []string, argName, newArg string) {
	for i := range args {
		if strings.HasPrefix(args[i], argName) {
			args[i] = newArg
		}
	}
}

func findStorageAddressArg(args []string) string {
	for i := range args {
		if strings.HasPrefix(args[i], storageAdvAddressArg) {
			return args[i]
		}
	}

	return ""
}

// newDeploymentFromDeployment takes a Deployment loaded from the Cluster and
// clears out the Metadata fields that are needed in the cluster.
func newDeploymentFromDeployment(src appsv1.Deployment) *appsv1.Deployment {
	depl := src.DeepCopy()
	depl.CreationTimestamp = metav1.Time{}

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
func updateNewDeployment(depl *appsv1.Deployment, shardSetName, shardName, oldDeploymentName, newDeploymentName string, labels map[string]string) error {
	// Add sharding labels
	if depl.ObjectMeta.Labels == nil {
		depl.ObjectMeta.Labels = map[string]string{}
	}

	depl.ObjectMeta.Labels = merge(
		labels,
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

		storageAdvAddress := ""
		if storageAddr := findStorageAddressArg(container.Args); storageAddr != "" {
			storageAdvAddress = strings.Replace(storageAddr, oldDeploymentName, newDeploymentName, 1)
		}

		if container.Name == "manager" {
			replaceArg(container.Args, ignoreShardsSelectorArg, selectorArgs)
			replaceArg(container.Args, storageAdvAddressArg, storageAdvAddress)
		}
	}

	// Update deployment name
	depl.ObjectMeta.Name = newDeploymentName

	// This makes the selector and template labels match.
	depl.Spec.Selector.MatchLabels = merge(
		labels,
		depl.Spec.Selector.MatchLabels,
	)

	depl.Spec.Template.ObjectMeta.Labels = merge(
		labels,
		depl.Spec.Template.ObjectMeta.Labels,
	)

	return nil
}

// newServiceFromService takes a Service loaded from the Cluster and
// clears out the Metadata fields that are needed in the cluster.
func newServiceFromService(src corev1.Service, newName string, labels, selector map[string]string) *corev1.Service {
	svc := src.DeepCopy()
	svc.CreationTimestamp = metav1.Time{}

	svc.ObjectMeta.Labels = merge(
		labels,
		svc.ObjectMeta.Labels,
	)

	svc.Spec.Selector = selector
	svc.Spec.ClusterIP = ""
	svc.Spec.ClusterIPs = nil

	// This is really a work around for the test cases
	// But it doesn't cost anything to do it here.
	// https://github.com/kubernetes-sigs/controller-runtime/issues?q=is%3Aissue+typemeta+empty+is%3Aclosed+
	svc.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}

	delete(svc.Annotations, "deployment.kubernetes.io/revision")

	svc.ObjectMeta.Name = newName
	svc.Generation = 0
	svc.ResourceVersion = ""
	svc.UID = ""
	svc.Status = corev1.ServiceStatus{}

	return svc
}

// return a copy of the "dest" map, with the elements of the "src" map applied
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
