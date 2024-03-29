package deploys

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	shardv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"github.com/weaveworks/flux-shard-controller/test"
)

const testControllerName = "kustomize-controller"

func TestNewDeploymentFromDeployment(t *testing.T) {
	depl := loadDeploymentFixture(t, "testdata/kustomize-controller.yaml")

	newDeploy := newDeploymentFromDeployment(*depl)

	want := loadDeploymentFixture(t, "testdata/kustomize-controller.golden.yaml")
	if diff := cmp.Diff(want, newDeploy); diff != "" {
		t.Fatalf("failed to generate new deployment:\n%s", diff)
	}
}

func TestGenerateDeployments(t *testing.T) {
	tests := []struct {
		name         string
		fluxShardSet *shardv1.FluxShardSet
		src          *appsv1.Deployment
		wantDeps     []*appsv1.Deployment
	}{
		{
			name: "generate when no shards are defined",
			fluxShardSet: &shardv1.FluxShardSet{
				Spec: shardv1.FluxShardSetSpec{
					SourceDeploymentRef: shardv1.SourceDeploymentReference{
						Name: testControllerName,
					},
					Shards: []shardv1.ShardSpec{},
				},
			},
			src: newTestDeployment(func(d *appsv1.Deployment) {
				d.Spec.Template.Spec.Containers[0].Args = []string{
					"--watch-label-selector=!sharding.fluxcd.io/key",
				}
			}),
			wantDeps: []*appsv1.Deployment{},
		},
		{
			name: "generation when one shard is defined",
			fluxShardSet: &shardv1.FluxShardSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-shard-set",
				},
				Spec: shardv1.FluxShardSetSpec{
					SourceDeploymentRef: shardv1.SourceDeploymentReference{
						Name: testControllerName,
					},
					Shards: []shardv1.ShardSpec{
						{
							Name: "shard-1",
						},
					},
				},
			},
			src: newTestDeployment(func(d *appsv1.Deployment) {
				d.ObjectMeta.Name = "kustomize-controller"
				d.Spec.Template.Spec.Containers[0].Args = []string{
					"--watch-label-selector=!sharding.fluxcd.io/key",
				}
			}),
			wantDeps: []*appsv1.Deployment{
				newTestDeployment(func(d *appsv1.Deployment) {
					d.Annotations = map[string]string{}
					d.ObjectMeta.Labels = map[string]string{
						"sharding.fluxcd.io/role":         "shard",
						"app.kubernetes.io/managed-by":    "flux-shard-controller",
						"templates.weave.works/shard":     "shard-1",
						"templates.weave.works/shard-set": "test-shard-set",
					}
					d.ObjectMeta.Name = "kustomize-controller-shard-1"
					d.Spec.Template.Spec.Containers[0].Args = []string{
						"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
					}
					d.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sharding.fluxcd.io/role":         "shard",
							"app":                             "kustomize-controller",
							"app.kubernetes.io/managed-by":    "flux-shard-controller",
							"templates.weave.works/shard-set": "test-shard-set",
							"templates.weave.works/shard":     "shard-1",
						},
					}
					d.Spec.Template.ObjectMeta.Labels = map[string]string{
						"sharding.fluxcd.io/role":         "shard",
						"app":                             "kustomize-controller",
						"app.kubernetes.io/managed-by":    "flux-shard-controller",
						"templates.weave.works/shard-set": "test-shard-set",
						"templates.weave.works/shard":     "shard-1",
					}
				}),
			},
		},
		{
			name: "generation when two shards is defined",
			fluxShardSet: &shardv1.FluxShardSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-shard-set",
				},
				Spec: shardv1.FluxShardSetSpec{
					SourceDeploymentRef: shardv1.SourceDeploymentReference{
						Name: testControllerName,
					},
					Shards: []shardv1.ShardSpec{
						{
							Name: "shard-a",
						},
						{
							Name: "shard-b",
						},
					},
				},
			},
			src: newTestDeployment(func(d *appsv1.Deployment) {
				d.Spec.Template.Spec.Containers[0].Args = []string{
					"--watch-label-selector=!sharding.fluxcd.io/key",
				}
			}),
			wantDeps: []*appsv1.Deployment{
				newTestDeployment(func(d *appsv1.Deployment) {
					d.Annotations = map[string]string{}
					d.ObjectMeta.Labels = test.ShardLabels("shard-a")
					d.ObjectMeta.Name = "kustomize-controller-shard-a"
					d.Spec.Template.Spec.Containers[0].Args = []string{
						"--watch-label-selector=sharding.fluxcd.io/key in (shard-a)",
					}
					d.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: test.ShardLabels("shard-a", map[string]string{
							"app": "kustomize-controller",
						}),
					}
					d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-a", map[string]string{
						"app": "kustomize-controller",
					})
				}),
				newTestDeployment(func(d *appsv1.Deployment) {
					d.Annotations = map[string]string{}
					d.ObjectMeta.Labels = test.ShardLabels("shard-b")
					d.ObjectMeta.Name = "kustomize-controller-shard-b"
					d.Spec.Template.Spec.Containers[0].Args = []string{
						"--watch-label-selector=sharding.fluxcd.io/key in (shard-b)",
					}
					d.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: test.ShardLabels("shard-b", map[string]string{
							"app": "kustomize-controller",
						}),
					}
					d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-b", map[string]string{
						"app": "kustomize-controller",
					})
				}),
			},
		},
		{
			name: "generation when the deployment has existing parameters",
			fluxShardSet: &shardv1.FluxShardSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-shard-set",
				},
				Spec: shardv1.FluxShardSetSpec{
					SourceDeploymentRef: shardv1.SourceDeploymentReference{
						Name: testControllerName,
					},
					Shards: []shardv1.ShardSpec{
						{
							Name: "shard-1",
						},
					},
				},
			},
			src: newTestDeployment(func(d *appsv1.Deployment) {
				d.Spec.Template.Spec.Containers[0].Args = []string{
					"--watch-all-namespaces=true",
					"--watch-label-selector=!sharding.fluxcd.io/key",
				}
			}),
			wantDeps: []*appsv1.Deployment{
				newTestDeployment(func(d *appsv1.Deployment) {
					d.Annotations = map[string]string{}
					d.ObjectMeta.Labels = test.ShardLabels("shard-1")
					d.ObjectMeta.Name = "kustomize-controller-shard-1"
					d.Spec.Template.Spec.Containers[0].Args = []string{
						"--watch-all-namespaces=true",
						"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
					}
					d.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: test.ShardLabels("shard-1", map[string]string{
							"app": "kustomize-controller",
						}),
					}
					d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-1", map[string]string{
						"app": "kustomize-controller",
					})
				}),
			},
		},
		{
			name: "generation when src deployment contains annotations",
			fluxShardSet: &shardv1.FluxShardSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-shard-set",
				},
				Spec: shardv1.FluxShardSetSpec{
					SourceDeploymentRef: shardv1.SourceDeploymentReference{
						Name: testControllerName,
					},
					Shards: []shardv1.ShardSpec{
						{
							Name: "shard-1",
						},
					},
				},
			},
			src: newTestDeployment(func(d *appsv1.Deployment) {
				d.ObjectMeta.Name = "kustomize-controller"
				d.Spec.Template.Spec.Containers[0].Args = []string{
					"--watch-label-selector=!sharding.fluxcd.io/key",
				}
				d.Annotations = map[string]string{
					"test-annot":                        "test",
					"deployment.kubernetes.io/revision": "test",
				}
			}),
			wantDeps: []*appsv1.Deployment{
				newTestDeployment(func(d *appsv1.Deployment) {
					d.Annotations = map[string]string{
						"test-annot": "test",
					}
					d.ObjectMeta.Labels = test.ShardLabels("shard-1")
					d.ObjectMeta.Name = "kustomize-controller-shard-1"
					d.Spec.Template.Spec.Containers[0].Args = []string{
						"--watch-label-selector=sharding.fluxcd.io/key in (shard-1)",
					}
					d.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: test.ShardLabels("shard-1", map[string]string{
							"app": "kustomize-controller",
						}),
					}
					d.Spec.Template.ObjectMeta.Labels = test.ShardLabels("shard-1", map[string]string{
						"app": "kustomize-controller",
					})
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generatedDeps, err := GenerateDeployments(tt.fluxShardSet, tt.src)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.wantDeps, generatedDeps); diff != "" {
				t.Fatalf("generated deployments dont match wanted: \n%s", diff)
			}
		})
	}
}

func TestGenerateDeployments_errors(t *testing.T) {
	// TODO Figure out what it means to be a flux controller and test for this
	tests := []struct {
		name         string
		fluxShardSet *shardv1.FluxShardSet
		src          *appsv1.Deployment
		wantErr      string
	}{
		{
			// The deployment does not have --watch-label-selector=
			name:    "deployment does not have sharding args",
			src:     newTestDeployment(),
			wantErr: "deployment flux-system/kustomize-controller is not configured to ignore sharding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateDeployments(tt.fluxShardSet, tt.src)

			if msg := err.Error(); msg != tt.wantErr {
				t.Fatalf("wanted error %q, got %q", tt.wantErr, msg)
			}
		})
	}
}

func newTestDeployment(opts ...func(*appsv1.Deployment)) *appsv1.Deployment {
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testControllerName,
			Namespace: "flux-system",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kustomize-controller",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kustomize-controller",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "manager",
							Args: []string{
								"--log-level=info",
								"--log-encoding=json",
								"--enable-leader-election",
							},
							Image: "ghcr.io/fluxcd/kustomize-controller:v0.35.1",
						},
					},
					ServiceAccountName: "kustomize-controller",
				},
			},
		},
	}

	for _, opt := range opts {
		opt(deploy)
	}

	return deploy
}

// This test is a test of the LabelSelector mechanism and could be removed.
func TestLabelSelectorShards(t *testing.T) {
	selectorTests := []struct {
		selector string
		labels   map[string]string
		ignore   bool
	}{
		{
			selector: "sharding.fluxcd.io/key notin (shard-1)",
			labels: map[string]string{
				"example.com/my-key":     "testing",
				"sharding.fluxcd.io/key": "test-1",
			},
			ignore: true,
		},
		{
			selector: "sharding.fluxcd.io/key notin (shard-1)",
			labels: map[string]string{
				"example.com/my-key":     "testing",
				"sharding.fluxcd.io/key": "shard-1",
			},
			ignore: false,
		},
		{
			selector: "!sharding.fluxcd.io/key",
			labels: map[string]string{
				"example.com/my-key":     "testing",
				"sharding.fluxcd.io/key": "shard-1",
			},
			ignore: false,
		},
	}

	for _, tt := range selectorTests {
		t.Run(tt.selector, func(t *testing.T) {
			s, err := labels.Parse(tt.selector)
			if err != nil {
				t.Fatal(err)
			}

			lbls := labels.Set(tt.labels)
			if m := s.Matches(lbls); m != tt.ignore {
				t.Fatalf("match %s against %v got %v, want %v", tt.selector, tt.labels, m, tt.ignore)
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
