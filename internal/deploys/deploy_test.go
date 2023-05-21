package deploys

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestDeploymentMatchesShardfn(t *testing.T) {
	fluxShardSet := &v1alpha1.FluxShardSet{}
	fluxShardSet.Spec.Shards = append(fluxShardSet.Spec.Shards, v1alpha1.ShardSpec{Name: "shard1"})

	deployment := appsv1.Deployment{}
	// matching false when it should match
	deployment.Name = "shard1"
	if !DeploymentMatchesShard(fluxShardSet, deployment) {
		t.Fatalf("failed to match shard1")
	}

	// Matching true when it shouldnt
	deployment.Name = "shard2"
	if DeploymentMatchesShard(fluxShardSet, deployment) {
		t.Fatalf("matched shard2")
	}
}

func TestMatchDeploymentsWithShards(t *testing.T) {
	deployments := []*appsv1.Deployment{}
	deployments = append(deployments, loadDeploymentFixture(t, "testdata/kustomize-controller.yaml"))
	deployments = append(deployments, loadDeploymentFixture(t, "testdata/kustomize-controller-2.yaml"))

	fluxShardSetFilename := "testdata/flux-shard-set.yaml"
	b, err := os.ReadFile(fluxShardSetFilename)
	if err != nil {
		t.Fatalf("failed to read flux-shard-set: %s", err)
	}

	fluxShardSet := &v1alpha1.FluxShardSet{}
	if err := yaml.Unmarshal(b, fluxShardSet); err != nil {
		t.Fatalf("failed to unmarshal YAML fluxshardset %s: %s", fluxShardSetFilename, err)
	}

	matchingDeps, err := GetDeploymentsMatchingFluxShardSet(fluxShardSet, deployments)
	if err != nil {
		t.Fatalf("failed to match deployments with shards: %s", err)
	}

	want := []*appsv1.Deployment{loadDeploymentFixture(t, "testdata/kustomize-controller.yaml"), loadDeploymentFixture(t, "testdata/kustomize-controller-2.yaml")}

	if diff := cmp.Diff(want, matchingDeps); diff != "" {
		t.Fatalf("failed to generate new deployment:\n%s", diff)
	}
}

func TestNewDeploymentFromDeployment(t *testing.T) {
	depl := loadDeploymentFixture(t, "testdata/kustomize-controller.yaml")

	newDeploy := NewDeploymentFromDeployment(*depl)

	want := loadDeploymentFixture(t, "testdata/kustomize-controller.golden.yaml")
	if diff := cmp.Diff(want, newDeploy); diff != "" {
		t.Fatalf("failed to generate new deployment:\n%s", diff)
	}
}

func loadDeploymentFixture(t *testing.T, filename string) *appsv1.Deployment {
	b, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read fixture: %s", err)
	}

	deploy := &appsv1.Deployment{}
	if err := yaml.Unmarshal(b, deploy); err != nil {
		t.Fatalf("failed to unmarshal YAML fixture %s: %s", filename, err)
	}

	return deploy
}
