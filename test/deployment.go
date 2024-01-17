package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

// DefaultNamespace is the namespace used for new resources, this can be
// overridden via an option.'
const DefaultNamespace = "flux-system"

// NewDeployment creates a new Deployment and apply the opts to it.
func NewDeployment(name string, opts ...func(*appsv1.Deployment)) *appsv1.Deployment {
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: DefaultNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:                pointer.Int32(1),
			ProgressDeadlineSeconds: pointer.Int32(600),
			RevisionHistoryLimit:    pointer.Int32(10),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				},
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"prometheus.io/port":   "8080",
						"prometheus.io/scrape": "true",
					},
					Labels: map[string]string{
						"app": name,
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
							Env: []corev1.EnvVar{
								{
									Name: "RUNTIME_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
							},
							Image:           "ghcr.io/fluxcd/kustomize-controller:v0.35.1",
							ImagePullPolicy: "IfNotPresent",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								ReadOnlyRootFilesystem: pointer.Bool(true),
								RunAsNonRoot:           pointer.Bool(true),
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http-prom",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "healthz",
									ContainerPort: 9440,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.IntOrString{Type: intstr.String, StrVal: "healthz"},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								PeriodSeconds:    10,
								SuccessThreshold: 1,
								TimeoutSeconds:   1,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/tmp",
									Name:      "temp",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: pointer.Int64(1337),
					},
					TerminationGracePeriodSeconds: pointer.Int64(60),
					RestartPolicy:                 "Always",
					DNSPolicy:                     "ClusterFirst",
					ServiceAccountName:            "kustomize-controller",
					DeprecatedServiceAccount:      "kustomize-controller",
					Volumes: []corev1.Volume{
						{
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
							Name: "temp",
						},
					},
					SchedulerName: "default-scheduler",
				},
			},
		},
	}

	for _, opt := range opts {
		opt(deploy)
	}

	return deploy
}

// NewServiceForDeployment creates a new Service with the correct labels
// for a Deployment and applies the opts to it.
func NewServiceForDeployment(deploy *appsv1.Deployment, opts ...func(*corev1.Service)) *corev1.Service {
	return NewService(deploy.GetName(), append(opts, func(svc *corev1.Service) {
		svc.ObjectMeta.Namespace = deploy.GetNamespace()
		svc.Spec.Selector = deploy.Spec.Selector.MatchLabels
	})...)
}

// NewService creates and returns a new Service.
func NewService(name string, opts ...func(*corev1.Service)) *corev1.Service {
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: DefaultNamespace,
		},
		Spec: corev1.ServiceSpec{
			IPFamilies: []corev1.IPFamily{
				corev1.IPv4Protocol,
			},
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					Name:       "http",
					TargetPort: intstr.FromString("http"),
				},
			},
		},
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc

}
