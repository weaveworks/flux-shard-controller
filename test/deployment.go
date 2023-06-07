package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// func MakeTestKustomization(name types.NamespacedName, opts ...func(*kustomizev1.Kustomization)) *kustomizev1.Kustomization {
// 	k := &kustomizev1.Kustomization{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "Kustomization",
// 			APIVersion: "kustomize.toolkit.fluxcd.io/v1beta2",
// 		},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name.Name,
// 			Namespace: name.Namespace,
// 		},
// 		Spec: kustomizev1.KustomizationSpec{
// 			Interval: metav1.Duration{Duration: 5 * time.Minute},
// 			Path:     "./examples/kustomize/environments/dev",
// 			Prune:    true,
// 			SourceRef: kustomizev1.CrossNamespaceSourceReference{
// 				Kind: "GitRepository",
// 				Name: "demo-repo",
// 			},
// 		},
// 	}

// 	for _, opt := range opts {
// 		opt(k)
// 	}

// 	return k
// }

func MakeTestDeployment(opts ...func(*appsv1.Deployment)) *appsv1.Deployment {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kustomize-controller",
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
