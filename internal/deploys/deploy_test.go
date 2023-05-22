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

	matchingDeps := GetDeploymentsMatchingFluxShardSet(fluxShardSet, deployments)

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

func TestGenerateDeployments(t *testing.T) {
	tests := []struct {
		name         string
		fluxShardSet v1alpha1.FluxShardSet
		deployments  []string
		want         []string
	}{
		{
			name: "generate deployment matching fluxshardset shard name",
			fluxShardSet: v1alpha1.FluxShardSet{
				Spec: v1alpha1.FluxShardSetSpec{
					Type: "kustomize",
					Shards: []v1alpha1.ShardSpec{
						{
							Name: "kustomize-controller",
						},
					},
				},
			},
			deployments: []string{
				"testdata/kustomize-controller.yaml",
				"testdata/kustomize-controller-2.yaml",
			},
			want: []string{
				"testdata/kustomize-controller.golden.yaml",
			},
		},
		{
			name: "no deployment matching fluxshardset shard name",
			fluxShardSet: v1alpha1.FluxShardSet{
				Spec: v1alpha1.FluxShardSetSpec{
					Type: "kustomize",
					Shards: []v1alpha1.ShardSpec{
						{
							Name: "shard-1",
						},
					},
				},
			},
			deployments: []string{
				"testdata/kustomize-controller.yaml",
				"testdata/kustomize-controller-2.yaml",
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := []*appsv1.Deployment{}
			for _, d := range tt.deployments {
				deps = append(deps, loadDeploymentFixture(t, d))
			}

			generatedDeps := GenerateDeployments(&tt.fluxShardSet, deps)

			wantDeps := []*appsv1.Deployment{}
			for _, d := range tt.want {
				wantDeps = append(wantDeps, loadDeploymentFixture(t, d))
			}

			if diff := cmp.Diff(wantDeps, generatedDeps); diff != "" {
				t.Fatalf("generated deployments dont match wanted: \n%s", diff)
			}

		})
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
