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
	"encoding/json"
	"fmt"
	"github.com/socialviolation/freyr/shared/openweather"
	"github.com/socialviolation/freyr/shared/trig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	freyrv1alpha1 "github.com/socialviolation/freyr/freyr/api/v1alpha1"
)

// FreyrReconciler reconciles a Freyr object
type FreyrReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=freyr.fmtl.au,resources=freyrs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=freyr.fmtl.au,resources=freyrs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=freyr.fmtl.au,resources=freyrs/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Freyr object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *FreyrReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// check if the Freyr exists
	freyrOp := &freyrv1alpha1.Freyr{}
	ns := req.Namespace
	if ns == "" {
		ns = "default"
		req.Namespace = "default"
		req.NamespacedName = types.NamespacedName{
			Name:      req.Name,
			Namespace: "default",
		}
	}

	err := r.Get(ctx, req.NamespacedName, freyrOp)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Freyr resource not deployed. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Freyr")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Freyr", "Name", freyrOp.Name, "Namespace", ns)

	captainUrl := fmt.Sprintf("http://captain-svc.%s.svc.cluster.local:80", ns)
	opJson, err := json.Marshal(freyrOp.Spec)
	if err != nil {
		log.Error(err, "Failed to marshal Freyr spec")
		return ctrl.Result{}, err
	}

	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: "config", Namespace: ns}, configMap)
	if err != nil && errors.IsNotFound(err) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config",
				Namespace: req.Namespace,
			},
			Data: map[string]string{
				"CAPTAIN_URL":     captainUrl,
				"OPERATOR_CONFIG": string(opJson),
			},
		}
		err = ctrl.SetControllerReference(freyrOp, configMap, r.Scheme)
		if err != nil {
			log.Error(err, "Failed to set controller reference")
			return ctrl.Result{}, err
		}

		err = r.Create(ctx, cm)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			return ctrl.Result{}, err
		}
		log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		return ctrl.Result{}, nil
	}

	// Check if the deployment already exists, if not create a new one
	captainDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "captain", Namespace: ns}, captainDep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForCaptain(freyrOp, configMap)
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
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Captain Deployment")
		return ctrl.Result{}, err
	}

	// Captain Service
	captainSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: "captain-svc", Namespace: ns}, captainSvc)
	if err != nil && errors.IsNotFound(err) {
		svc := r.serviceForCaptain(freyrOp, captainDep)
		log.Info("Creating a new Captain Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Conscript
	conscriptDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: "conscript", Namespace: ns}, conscriptDep)
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
			log.Error(err, "Failed to update ConfigMap", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
			return ctrl.Result{}, err
		}

		if captainDep.Spec.Template.ObjectMeta.Annotations == nil {
			captainDep.Spec.Template.ObjectMeta.Annotations = map[string]string{}
		}

		captainDep.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		err = r.Update(ctx, captainDep)
		return ctrl.Result{}, nil
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
		return ctrl.Result{}, nil
	}

	targetConscripts := int32(1)
	if freyrOp.Spec.Mode == "weather" {
		l := openweather.Location{
			Country: freyrOp.Spec.Weather.Country,
			City:    freyrOp.Spec.Weather.City,
		}
		llt, err := openweather.GetTempByCountry(freyrOp.Spec.Weather.APIKey, l)
		if err != nil {

			log.Error(err, "Failed to retrieve weather")
		}
		targetConscripts = llt.Temp
		log.Info("Reconciling Weather mode", "conscripts", targetConscripts)
	} else if freyrOp.Spec.Mode == "trig" {
		args := trig.Args{
			Duration: freyrOp.Spec.Trig.Duration,
			Min:      freyrOp.Spec.Trig.Min,
			Max:      freyrOp.Spec.Trig.Max,
		}
		fv, err := trig.GetValue(args)
		if err != nil {
			log.Error(err, "Failed to retrieve trig value")
		} else {
			targetConscripts = int32(fv)
		}
		log.Info("Reconciling Trig mode", "target", targetConscripts, "actual", conscriptDep.Spec.Replicas, "duration", freyrOp.Spec.Trig.Duration, "min", freyrOp.Spec.Trig.Min, "max", freyrOp.Spec.Trig.Max)
	}

	if *conscriptDep.Spec.Replicas != targetConscripts {
		conscriptDep.Spec.Replicas = &targetConscripts
		err = r.Update(ctx, conscriptDep)
		if err != nil {
			log.Error(err, "Failed to update Conscript Deployment", "Deployment.Namespace", conscriptDep.Namespace, "Deployment.Name", conscriptDep.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreyrReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := false
	return ctrl.NewControllerManagedBy(mgr).
		For(&freyrv1alpha1.Freyr{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			NeedLeaderElection:      &b,
		}).
		Complete(r)
}

func labelsForCaptain() map[string]string {
	return map[string]string{"app": "captain"}
}

func (r *FreyrReconciler) deploymentForCaptain(c *freyrv1alpha1.Freyr, config *corev1.ConfigMap) *appsv1.Deployment {
	replicas := int32(1)
	ls := labelsForCaptain()

	if c.Spec.Captain.Image == "" {
		c.Spec.Captain.Image = "australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest"
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "captain",
			Namespace: config.Namespace,
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
						Image: c.Spec.Captain.Image,
						Name:  "captain",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5001,
						}},
						ImagePullPolicy: corev1.PullIfNotPresent,
						//Resources: corev1.ResourceRequirements{
						//	Limits: corev1.ResourceList{
						//		corev1.ResourceCPU:    resource.MustParse("200m"),
						//		corev1.ResourceMemory: resource.MustParse("512Mi"),
						//	},
						//},
						Env: []corev1.EnvVar{},
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: config.Name,
								},
							},
						}},
					}},
				},
			},
		},
	}

	for k, v := range c.Spec.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	for k, v := range c.Spec.Captain.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	err := ctrl.SetControllerReference(c, dep, r.Scheme)
	if err != nil {
		return nil
	}

	return dep
}

func (r *FreyrReconciler) serviceForCaptain(c *freyrv1alpha1.Freyr, d *appsv1.Deployment) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "captain-svc",
			Namespace: d.Namespace,
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

func (r *FreyrReconciler) deploymentForConscript(c *freyrv1alpha1.Freyr, svc *corev1.Service) *appsv1.Deployment {
	replicas := int32(1)
	ls := labelsForConscript()
	if c.Spec.Conscript.Image == "" {
		c.Spec.Conscript.Image = "australia-southeast2-docker.pkg.dev/freyr-operator/imgs/conscript:latest"
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "conscript",
			Namespace: svc.Namespace,
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
						Image: c.Spec.Conscript.Image,
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
								Name:  "CAPTAIN_URL",
								Value: fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, svc.Spec.Ports[0].Port),
							},
						},
					}},
				},
			},
		},
	}

	for k, v := range c.Spec.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	for k, v := range c.Spec.Conscript.EnvVars {
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
	}

	err := ctrl.SetControllerReference(c, dep, r.Scheme)
	if err != nil {
		return nil
	}
	return dep
}
