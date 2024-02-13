/*
Copyright 2024 nick@fmtl.au.

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
	"fmt"
	freyrv1alpha1 "github.com/socialviolation/freyr/op_freyr/api/v1alpha1"
	"github.com/socialviolation/freyr/op_freyr/internal/openweather"
	"github.com/socialviolation/freyr/op_freyr/internal/trig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// OperatorReconciler reconciles a Operator object
type OperatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=freyr.fmtl.au,resources=operators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=freyr.fmtl.au,resources=operators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=freyr.fmtl.au,resources=operators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// check if the Operator exists
	freyrOp := &freyrv1alpha1.Operator{}
	err := r.Get(ctx, req.NamespacedName, freyrOp)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not captainDep, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Freyr resource not deployed. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Freyr")
		return ctrl.Result{}, err
	}

	targetConscripts := int32(1)
	if freyrOp.Spec.Mode == "weather" {
		llt, err := openweather.GetTempByCountry(freyrOp.Spec.Weather.APIKey, freyrOp.Spec.Weather.Country, freyrOp.Spec.Weather.City)
		if err != nil {
			log.Error(err, "Failed to retrieve weather")
		}
		targetConscripts = llt.Temp
	} else if freyrOp.Spec.Mode == "trig" {
		targetConscripts = trig.GetValue(freyrOp.Spec.Trig.Period, freyrOp.Spec.Trig.Min, freyrOp.Spec.Trig.Max)
	}

	// Check if the deployment already exists, if not create a new one
	captainDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "captain", Namespace: freyrOp.Namespace}, captainDep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForCaptain(freyrOp)
		if dep == nil {
			log.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Creating a new Captain Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Captain Deployment")
		return ctrl.Result{}, err
	}

	// Captain Service
	captainSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: "captain-svc", Namespace: freyrOp.Namespace}, captainSvc)
	if err != nil && errors.IsNotFound(err) {
		svc := r.serviceForCaptain(freyrOp, captainDep)
		log.Info("Creating a new Captain Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Conscript
	conscriptDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "conscript", Namespace: freyrOp.Namespace}, conscriptDep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForConscript(freyrOp, captainSvc)
		if dep == nil {
			log.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Creating a new Conscript Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Conscript Deployment")
		return ctrl.Result{}, err
	}

	if *captainDep.Spec.Replicas != 1 {
		rep := int32(1)
		captainDep.Spec.Replicas = &rep
		err = r.Update(ctx, captainDep)
		if err != nil {
			log.Error(err, "Failed to update Captain Deployment", "Deployment.Namespace", captainDep.Namespace, "Deployment.Name", captainDep.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	if *conscriptDep.Spec.Replicas != targetConscripts {
		conscriptDep.Spec.Replicas = &targetConscripts
		err = r.Update(ctx, conscriptDep)
		if err != nil {
			log.Error(err, "Failed to update Conscript Deployment", "Deployment.Namespace", conscriptDep.Namespace, "Deployment.Name", conscriptDep.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&freyrv1alpha1.Operator{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func labelsForCaptain() map[string]string {
	return map[string]string{"app": "captain"}
}

func (r *OperatorReconciler) deploymentForCaptain(c *freyrv1alpha1.Operator) *appsv1.Deployment {
	replicas := int32(1)
	ls := labelsForCaptain()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "captain",
			Namespace: c.Namespace,
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
						Image: "australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest",
						Name:  "captain",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5001,
						}},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					}},
				},
			},
		},
	}

	err := ctrl.SetControllerReference(c, dep, r.Scheme)
	if err != nil {
		return nil
	}
	return dep
}

func (r *OperatorReconciler) serviceForCaptain(c *freyrv1alpha1.Operator, d *appsv1.Deployment) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "captain-svc",
			Namespace: c.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labelsForCaptain(),
			Ports: []corev1.ServicePort{{
				Name:     "http",
				Protocol: "TCP",
				Port:     int32(80),
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: d.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort,
				},
			}},
		},
	}

	err := ctrl.SetControllerReference(c, svc, r.Scheme)
	if err != nil {
		return nil
	}

	return svc
}

func labelsForConscript() map[string]string {
	return map[string]string{"app": "conscript"}
}

func (r *OperatorReconciler) deploymentForConscript(c *freyrv1alpha1.Operator, svc *corev1.Service) *appsv1.Deployment {
	replicas := int32(1)
	ls := labelsForConscript()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conscript",
			Namespace: c.Namespace,
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
						Image: "australia-southeast2-docker.pkg.dev/freyr-operator/imgs/conscript:latest",
						Name:  "conscript",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5003,
						}},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
						Env: []corev1.EnvVar{{
							Name:  "CAPTAIN_URL",
							Value: fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, svc.Spec.Ports[0].Port),
						}},
					}},
				},
			},
		},
	}

	err := ctrl.SetControllerReference(c, dep, r.Scheme)
	if err != nil {
		return nil
	}
	return dep
}
