/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/berndonline/k8s-helloworld-operator/api/v1alpha1"
)

// OperatorReconciler reconciles a Operator object
type OperatorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// IntOrString integer or string
type IntOrString struct {
	Type   Type   `protobuf:"varint,1,opt,name=type,casttype=Type"`
	IntVal int32  `protobuf:"varint,2,opt,name=intVal"`
	StrVal string `protobuf:"bytes,3,opt,name=strVal"`
}

// Type represents the stored type of IntOrString.
type Type int

// Int - Type
const (
	Int intstr.Type = iota
	String
)

// +kubebuilder:rbac:groups=app.helloworld.io,resources=operators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.helloworld.io,resources=operators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.helloworld.io,resources=operators/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Operator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	// Fetch the Operator instance
	operator := &appv1alpha1.Operator{}
	err := r.Get(ctx, req.NamespacedName, operator)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Operator resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Operator")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: operator.Name, Namespace: operator.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForOperator(operator)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Check if the deployment Spec.Template, matches the found Spec.Template
	deploy := r.deploymentForOperator(operator)
	if !equality.Semantic.DeepDerivative(deploy.Spec.Template, found.Spec.Template) {
		found = deploy
		log.Info("Updating Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
		err := r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Ensure the deployment size is the same as the spec
	size := operator.Spec.Size
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Check if the configmap already exists, if not create a new one
	foundConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: operator.Name, Namespace: operator.Namespace}, foundConfigMap)
	if err != nil && errors.IsNotFound(err) {
		// Define a new ingress
		cm := r.configMapForOperator(operator)
		log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		err = r.Create(ctx, cm)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			return ctrl.Result{}, err
		}

		// ConfigMap created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// Check if the ConfigMap Data, matches the found Data
	configMap := r.configMapForOperator(operator)
	if !equality.Semantic.DeepDerivative(configMap.Data, foundConfigMap.Data) {
		foundConfigMap = configMap
		log.Info("Updating ConfigMap", "ConfigMap.Namespace", foundConfigMap.Namespace, "ConfigMap.Name", foundConfigMap.Name)
		err := r.Update(ctx, foundConfigMap)
		if err != nil {
			log.Error(err, "Failed to update ConfigMap", "ConfigMap.Namespace", foundConfigMap.Namespace, "ConfigMap.Name", foundConfigMap.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if the service already exists, if not create a new one
	foundService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: operator.Name, Namespace: operator.Namespace}, foundService)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		dep := r.serviceForOperator(operator)
		log.Info("Creating a new Service", "Service.Namespace", dep.Namespace, "Service.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", dep.Namespace, "Service.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Check if the ingress already exists, if not create a new one
	foundIngress := &networkingv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: operator.Name, Namespace: operator.Namespace}, foundIngress)
	if err != nil && errors.IsNotFound(err) {
		// Define a new ingress
		dep := r.ingressForOperator(operator)
		log.Info("Creating a new Ingress", "Ingress.Namespace", dep.Namespace, "Ingress.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Ingress", "Ingress.Namespace", dep.Namespace, "Ingress.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Ingress created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Ingress")
		return ctrl.Result{}, err
	}

	// Update the Operator status with the pod names
	// List the pods for this operator's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(found.Namespace),
		client.MatchingLabels(labelsForOperator(found.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Operator.Namespace", found.Namespace, "Operator.Name", found.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, operator.Status.Nodes) {
		operator.Status.Nodes = podNames
		err := r.Status().Update(ctx, operator)
		if err != nil {
			log.Error(err, "Failed to update Operator status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deploymentForOperator returns a operator Deployment object
func (r *OperatorReconciler) deploymentForOperator(m *appv1alpha1.Operator) *appsv1.Deployment {
	ls := labelsForOperator(m.Name)
	replicas := m.Spec.Size
	servers := strings.Join(m.Spec.DBservers, ",")
	args := m.Spec.JaegerCollector

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           "jaegertracing/jaeger-agent:latest",
							ImagePullPolicy: corev1.PullAlways,
							Name:            "jaeger-agent",
							Ports: []corev1.ContainerPort{{
								ContainerPort: 5775,
								Protocol:      corev1.ProtocolUDP,
								Name:          "zk-compact-trft",
							},
								{
									ContainerPort: 5778,
									Protocol:      corev1.ProtocolTCP,
									Name:          "config-rest",
								},
								{
									ContainerPort: 6831,
									Protocol:      corev1.ProtocolUDP,
									Name:          "jg-compact-trft",
								},
								{
									ContainerPort: 6832,
									Protocol:      corev1.ProtocolUDP,
									Name:          "jg-binary-trft",
								},
								{
									ContainerPort: 14271,
									Protocol:      corev1.ProtocolTCP,
									Name:          "admin-http",
								}},
							Args: args,
						},
						{
							Image:           m.Spec.Image,
							ImagePullPolicy: corev1.PullAlways,
							Name:            "helloworld",
							Ports: []corev1.ContainerPort{{
								ContainerPort: 8080,
								Name:          "operator",
							}},
							Env: []corev1.EnvVar{
								{
									Name:  "RESPONSE",
									Value: m.Spec.Response,
								},
								{
									Name:  "MONGODB",
									Value: strconv.FormatBool(m.Spec.MongoDB),
								},
								{
									Name:  "DBSERVERS",
									Value: servers,
								},
								{
									Name:  "DATABASE",
									Value: m.Spec.Database,
								},
								{
									Name:  "DBUSER",
									Value: m.Spec.DBuser,
								},
								{
									Name:  "DBPASS",
									Value: m.Spec.DBpass,
								},
							},
							EnvFrom: []corev1.EnvFromSource{{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.Name,
									},
								},
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      m.Name,
								ReadOnly:  true,
								MountPath: "/helloworld/",
							}},
						}},
					Volumes: []corev1.Volume{{
						Name: m.Name,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: m.Name,
								},
							},
						},
					}},
				},
			},
		},
	}

	// Set Operator instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// configMapForOperator returns a operator Service object
func (r *OperatorReconciler) configMapForOperator(m *appv1alpha1.Operator) *corev1.ConfigMap {
	ls := labelsForOperator(m.Name)

	dep := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Data: map[string]string{"helloworld.txt": m.Spec.Response},
	}
	// Set Operator instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// serviceForOperator returns a operator Service object
func (r *OperatorReconciler) serviceForOperator(m *appv1alpha1.Operator) *corev1.Service {
	ls := labelsForOperator(m.Name)

	dep := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{{
				// corev1.ServicePort{
				Port: 80,
				Name: "port80",
				TargetPort: intstr.IntOrString{
					IntVal: 8080,
					Type:   intstr.Int,
				},
			}},
			Type: "ClusterIP",
		},
	}
	// Set Operator instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// serviceForOperator returns a operator Ingress object
func (r *OperatorReconciler) ingressForOperator(m *appv1alpha1.Operator) *networkingv1.Ingress {
	// ls := labelsForOperator(m.Name)

	ingressPaths := []networkingv1.HTTPIngressPath{
		networkingv1.HTTPIngressPath{
			Path:     "/" + m.Name + "(/|$)(.*)",
			PathType: func() *networkingv1.PathType { s := networkingv1.PathTypeImplementationSpecific; return &s }(),
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: m.Name,
					Port: networkingv1.ServiceBackendPort{
						Number: 80,
					},
				},
			},
		},
	}
	ingressSpec := networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: "",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: ingressPaths,
					},
				},
			},
		},
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
			},
		},
		Spec: ingressSpec,
	}

	// Set Operator instance as the owner and controller
	ctrl.SetControllerReference(m, ingress, r.Scheme)
	return ingress
}

// labelsForOperator returns the labels for selecting the resources
// belonging to the given operator CR name.
func labelsForOperator(name string) map[string]string {
	return map[string]string{"app": "operator", "operator_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *OperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.Operator{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
