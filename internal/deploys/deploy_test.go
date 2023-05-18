package deploys

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

// NewDeploymentFromDeployment takes a Deployment loaded from the Cluster and
// clears out the Metadata fields that are needed in the cluster.
func NewDeploymentFromDeployment(src appsv1.Deployment) *appsv1.Deployment {
	// remove or clear
	// creationTimestamp
	// deployment.kubernetes.io annotation
	// generation
	// resourceVersion
	// uid
	// status

	// add managed-by label TODO: find out what this label is.

	return &src
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
