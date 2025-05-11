/*
Copyright 2025.

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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/socialviolation/freyr/shared/openweather"
	"github.com/socialviolation/freyr/shared/trig"
	freyrv1alpha1 "github.com/socialviolation/freyr/ship-operator/api/v1alpha1"
)

// ShipReconciler reconciles a Ship object
type ShipReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=freyr.fmtl.au,resources=ships,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=freyr.fmtl.au,resources=ships/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=freyr.fmtl.au,resources=ships/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Ship object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ShipReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	ship := &freyrv1alpha1.Ship{}
	ns := req.Namespace
	if ns == "" {
		return ctrl.Result{}, fmt.Errorf("namespace is empty")
	}

	err := r.Get(ctx, req.NamespacedName, ship)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Ship resource not deployed. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Ship")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Ship")

	captainUrl := fmt.Sprintf("http://captain-svc.%s.svc.cluster.local:80", ns)
	opJson, err := json.Marshal(ship.Spec)
	if err != nil {
		log.Error(err, "Failed to marshal Ship spec")
		return ctrl.Result{}, err
	}

	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: "config", Namespace: ns}, configMap)
	if err != nil && errors.IsNotFound(err) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config",
				Namespace: req.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "ship-operator",
					"app.kubernetes.io/owner":      ship.GetName(),
					"app.kubernetes.io/owner-ns":   ship.GetNamespace(),
				},
			},
			Data: map[string]string{
				"CAPTAIN_URL":     captainUrl,
				"OPERATOR_CONFIG": string(opJson),
			},
		}

		err = r.Create(ctx, cm)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap")
			return ctrl.Result{}, err
		}
		log.Info("Created a new ConfigMap")
		return ctrl.Result{}, nil
	}

	captainDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "captain", Namespace: ns}, captainDep)
	if err != nil && errors.IsNotFound(err) {
		dep := r.deploymentForCaptain(ship, configMap)
		if dep == nil {
			log.Error(err, "Failed to create new Captain Deployment")
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Captain Deployment")
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Captain Deployment")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Captain Deployment")
		return ctrl.Result{}, err
	}

	// Captain Service
	captainSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: "captain-svc", Namespace: ns}, captainSvc)
	if err != nil && errors.IsNotFound(err) {
		svc := r.serviceForCaptain(ship, captainDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		log.Info("Creating a new Captain Service")
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Captain Service")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Captain Service")
		return ctrl.Result{}, err
	}

	// Conscript
	conscriptDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "conscript", Namespace: ns}, conscriptDep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForConscript(ship)
		if dep == nil {
			log.Error(err, "Failed to create new Conscript Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Creating a new Conscript Deployment")
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Conscript Deployment")
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Conscript Deployment")
		return ctrl.Result{}, err
	}

	if configMap.Data["CAPTAIN_URL"] != captainUrl || configMap.Data["OPERATOR_CONFIG"] != string(opJson) {
		log.Info("Updating ConfigMap")
		configMap.Data["CAPTAIN_URL"] = captainUrl
		configMap.Data["OPERATOR_CONFIG"] = string(opJson)
		err = r.Update(ctx, configMap)
		if err != nil {
			log.Error(err, "Failed to update ConfigMap")
			return ctrl.Result{Requeue: false}, err
		}

		if captainDep.Spec.Template.ObjectMeta.Annotations == nil {
			captainDep.Spec.Template.ObjectMeta.Annotations = map[string]string{}
		}

		captainDep.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		err = r.Update(ctx, captainDep)
		if err != nil {
			log.Error(err, "Failed to update Captain Deployment")
			return ctrl.Result{}, err
		}
	}

	if *captainDep.Spec.Replicas != 1 {
		rep := int32(1)
		captainDep.Spec.Replicas = &rep
		err = r.Update(ctx, captainDep)
		if err != nil {
			log.Error(err, "Failed to update Captain Deployment")
			return ctrl.Result{}, err
		}
	}

	targetConscripts := int32(1)
	if ship.Spec.Mode == "weather" {
		l := openweather.Location{
			Country: ship.Spec.Weather.Country,
			City:    ship.Spec.Weather.City,
		}
		llt, err := openweather.GetTempByCountry(ship.Spec.Weather.APIKey, l)
		if err != nil {
			log.Error(err, "Failed to retrieve weather")
		}
		targetConscripts = llt.Temp
		log.Info("Reconciling Weather mode", "conscripts", targetConscripts)
	} else if ship.Spec.Mode == "trig" {
		args := trig.Args{
			Duration: ship.Spec.Trig.Duration,
			Min:      ship.Spec.Trig.Min,
			Max:      ship.Spec.Trig.Max,
		}
		fv, err := trig.GetValue(args)
		if err != nil {
			log.Error(err, "Failed to retrieve trig value")
		} else {
			targetConscripts = int32(fv)
		}
		log.Info("Reconciling Trig mode", "target", targetConscripts, "actual", conscriptDep.Spec.Replicas, "duration", ship.Spec.Trig.Duration, "min", ship.Spec.Trig.Min, "max", ship.Spec.Trig.Max)
	}

	if *conscriptDep.Spec.Replicas != targetConscripts {
		conscriptDep.Spec.Replicas = &targetConscripts
		err = r.Update(ctx, conscriptDep)
		if err != nil {
			log.Error(err, "Failed to update Conscript Deployment")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func safeSetControllerReference(owner, object client.Object, scheme *runtime.Scheme) error {
	if object.GetNamespace() != "" {
		return controllerutil.SetControllerReference(owner, object, scheme)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ShipReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := false
	return ctrl.NewControllerManagedBy(mgr).
		For(&freyrv1alpha1.Ship{}).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(IgnoreReplicasOnlyUpdate)).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			NeedLeaderElection:      &b,
		}).
		Complete(r)
}

func (r *ShipReconciler) deploymentForCaptain(ship *freyrv1alpha1.Ship, config *corev1.ConfigMap) *appsv1.Deployment {
	replicas := int32(1)
	ls := map[string]string{
		"app":                          "captain",
		"app.kubernetes.io/managed-by": "ship-operator",
		"app.kubernetes.io/owner":      ship.GetName(),
		"app.kubernetes.io/owner-ns":   ship.GetNamespace(),
	}

	if ship.Spec.Captain.Image == "" {
		ship.Spec.Captain.Image = "australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest"
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "captain",
			Namespace: ship.GetNamespace(),
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
					Containers: []corev1.Container{{
						Image: ship.Spec.Captain.Image,
						Name:  "captain",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5001,
						}},
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
						Env: []corev1.EnvVar{},
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "config",
								},
							},
						}},
					}},
				},
			},
		},
	}

	for k, v := range ship.Spec.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	for k, v := range ship.Spec.Captain.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	err := safeSetControllerReference(ship, dep, r.Scheme)
	if err != nil {
		return nil
	}

	return dep
}

func (r *ShipReconciler) serviceForCaptain(ship *freyrv1alpha1.Ship, containerPort int32) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "captain-svc",
			Namespace: ship.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":                          "captain",
				"app.kubernetes.io/managed-by": "ship-operator",
				"app.kubernetes.io/owner":      ship.GetName(),
				"app.kubernetes.io/owner-ns":   ship.GetNamespace(),
			},
			Ports: []corev1.ServicePort{{
				Name:     "http",
				Protocol: "TCP",
				Port:     int32(80),
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: containerPort,
				},
			}},
		},
	}

	err := safeSetControllerReference(ship, svc, r.Scheme)
	if err != nil {
		return nil
	}

	return svc
}

func (r *ShipReconciler) deploymentForConscript(ship *freyrv1alpha1.Ship) *appsv1.Deployment {
	replicas := int32(1)
	ls := map[string]string{
		"app":                          "conscript",
		"app.kubernetes.io/managed-by": "ship-operator",
		"app.kubernetes.io/owner":      ship.GetName(),
		"app.kubernetes.io/owner-ns":   ship.GetNamespace(),
	}
	if ship.Spec.Conscript.Image == "" {
		ship.Spec.Conscript.Image = "australia-southeast2-docker.pkg.dev/freyr-operator/imgs/conscript:latest"
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conscript",
			Namespace: ship.GetNamespace(),
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    ls,
					Name:      "conscript",
					Namespace: ship.GetNamespace(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: ship.Spec.Conscript.Image,
						Name:  "conscript",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5003,
						}},
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "CAPTAIN_URL",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "config",
										},
										Key: "CAPTAIN_URL",
									},
								},
							},
						},
					}},
				},
			},
		},
	}

	for k, v := range ship.Spec.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	for k, v := range ship.Spec.Conscript.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	err := safeSetControllerReference(ship, dep, r.Scheme)
	if err != nil {
		return nil
	}

	return dep
}

var IgnoreReplicasOnlyUpdate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldDep, okOld := e.ObjectOld.(*appsv1.Deployment)
		newDep, okNew := e.ObjectNew.(*appsv1.Deployment)
		if !okOld || !okNew {
			// can't determine change, play safe
			return true
		}

		// Create deep copies and zero out replicas for comparison
		oldCopy := oldDep.DeepCopy()
		newCopy := newDep.DeepCopy()

		// Ignore differences in .spec.replicas
		oldCopy.Spec.Replicas = nil
		newCopy.Spec.Replicas = nil

		return !reflect.DeepEqual(oldCopy.Spec, newCopy.Spec)
	},
}
