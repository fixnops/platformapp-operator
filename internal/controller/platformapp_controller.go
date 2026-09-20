/*
Copyright 2026.

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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/fixnops/platformapp-operator/api/v1alpha1"
)

const (
	conditionAvailable   = "Available"
	conditionProgressing = "Progressing"
	conditionDegraded    = "Degraded"
)

// PlatformAppReconciler reconciles a PlatformApp object.
type PlatformAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the actual state toward the state requested by PlatformApp.
func (r *PlatformAppReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	platformApp := &appsv1alpha1.PlatformApp{}

	if err := r.Get(ctx, req.NamespacedName, platformApp); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(
				"PlatformApp resource was not found; ignoring because it may have been deleted",
				"namespace", req.Namespace,
				"name", req.Name,
			)
			return ctrl.Result{}, nil
		}

		log.Error(
			err,
			"Failed to get PlatformApp",
			"namespace", req.Namespace,
			"name", req.Name,
		)
		return ctrl.Result{}, err
	}

	desiredReplicas := int32(1)
	if platformApp.Spec.Replicas != nil {
		desiredReplicas = *platformApp.Spec.Replicas
	}

	log.Info(
		"Reconciling PlatformApp",
		"namespace", platformApp.Namespace,
		"name", platformApp.Name,
		"image", platformApp.Spec.Image,
		"replicas", desiredReplicas,
		"port", platformApp.Spec.Port,
		"generation", platformApp.Generation,
	)

	labels := map[string]string{
		"app.kubernetes.io/name":       "platformapp",
		"app.kubernetes.io/instance":   platformApp.Name,
		"app.kubernetes.io/managed-by": "platformapp-operator",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      platformApp.Name,
			Namespace: platformApp.Namespace,
		},
	}

	deploymentOperation, err := controllerutil.CreateOrUpdate(
		ctx,
		r.Client,
		deployment,
		func() error {
			deployment.Labels = labels
			deployment.Spec.Replicas = &desiredReplicas
			deployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: labels,
			}
			deployment.Spec.Template.ObjectMeta.Labels = labels
			deployment.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "application",
					Image: platformApp.Spec.Image,
					Ports: []corev1.ContainerPort{
						{
							Name:          "app-port",
							ContainerPort: platformApp.Spec.Port,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			}

			return controllerutil.SetControllerReference(
				platformApp,
				deployment,
				r.Scheme,
			)
		},
	)
	if err != nil {
		log.Error(
			err,
			"Failed to reconcile Deployment",
			"namespace", platformApp.Namespace,
			"name", platformApp.Name,
		)
		return ctrl.Result{}, err
	}

	log.Info(
		"Reconciled Deployment",
		"namespace", deployment.Namespace,
		"name", deployment.Name,
		"operation", deploymentOperation,
	)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      platformApp.Name,
			Namespace: platformApp.Namespace,
		},
	}

	serviceOperation, err := controllerutil.CreateOrUpdate(
		ctx,
		r.Client,
		service,
		func() error {
			service.Labels = labels
			service.Spec.Selector = labels
			service.Spec.Type = corev1.ServiceTypeClusterIP
			service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "app-port",
					Port:       platformApp.Spec.Port,
					TargetPort: intstr.FromInt(int(platformApp.Spec.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			}

			return controllerutil.SetControllerReference(
				platformApp,
				service,
				r.Scheme,
			)
		},
	)
	if err != nil {
		log.Error(
			err,
			"Failed to reconcile Service",
			"namespace", platformApp.Namespace,
			"name", platformApp.Name,
		)
		return ctrl.Result{}, err
	}

	log.Info(
		"Reconciled Service",
		"namespace", service.Namespace,
		"name", service.Name,
		"operation", serviceOperation,
	)

	statusChanged, err := r.updateStatus(
		ctx,
		platformApp,
		deployment,
		desiredReplicas,
	)
	if err != nil {
		log.Error(
			err,
			"Failed to update PlatformApp status",
			"namespace", platformApp.Namespace,
			"name", platformApp.Name,
		)
		return ctrl.Result{}, err
	}

	if statusChanged {
		log.Info(
			"Updated PlatformApp status",
			"namespace", platformApp.Namespace,
			"name", platformApp.Name,
			"observedGeneration", platformApp.Status.ObservedGeneration,
			"readyReplicas", platformApp.Status.ReadyReplicas,
		)
	}

	return ctrl.Result{}, nil
}

// updateStatus calculates and writes the observed PlatformApp state.
//
// It returns true when a Kubernetes status patch was necessary. Avoiding an
// unnecessary patch prevents stable objects from continuously reconciling
// because of their own status updates.
func (r *PlatformAppReconciler) updateStatus(
	ctx context.Context,
	platformApp *appsv1alpha1.PlatformApp,
	deployment *appsv1.Deployment,
	desiredReplicas int32,
) (bool, error) {
	statusBeforeChange := platformApp.DeepCopy()

	readyReplicas := deployment.Status.ReadyReplicas
	isAvailable := readyReplicas >= desiredReplicas

	platformApp.Status.ObservedGeneration = platformApp.Generation
	platformApp.Status.ReadyReplicas = readyReplicas

	if isAvailable {
		apimeta.SetStatusCondition(
			&platformApp.Status.Conditions,
			metav1.Condition{
				Type:               conditionAvailable,
				Status:             metav1.ConditionTrue,
				Reason:             "DeploymentAvailable",
				Message:            fmt.Sprintf("%d of %d requested replicas are ready", readyReplicas, desiredReplicas),
				ObservedGeneration: platformApp.Generation,
			},
		)

		apimeta.SetStatusCondition(
			&platformApp.Status.Conditions,
			metav1.Condition{
				Type:               conditionProgressing,
				Status:             metav1.ConditionFalse,
				Reason:             "DeploymentComplete",
				Message:            "All requested replicas are ready",
				ObservedGeneration: platformApp.Generation,
			},
		)
	} else {
		apimeta.SetStatusCondition(
			&platformApp.Status.Conditions,
			metav1.Condition{
				Type:               conditionAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ReplicasNotReady",
				Message:            fmt.Sprintf("%d of %d requested replicas are ready", readyReplicas, desiredReplicas),
				ObservedGeneration: platformApp.Generation,
			},
		)

		apimeta.SetStatusCondition(
			&platformApp.Status.Conditions,
			metav1.Condition{
				Type:               conditionProgressing,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForReplicas",
				Message:            fmt.Sprintf("Waiting for %d requested replicas to become ready", desiredReplicas),
				ObservedGeneration: platformApp.Generation,
			},
		)
	}

	apimeta.SetStatusCondition(
		&platformApp.Status.Conditions,
		metav1.Condition{
			Type:               conditionDegraded,
			Status:             metav1.ConditionFalse,
			Reason:             "ReconciliationSucceeded",
			Message:            "Deployment and Service reconciliation succeeded",
			ObservedGeneration: platformApp.Generation,
		},
	)

	if reflect.DeepEqual(
		statusBeforeChange.Status,
		platformApp.Status,
	) {
		return false, nil
	}

	if err := r.Status().Patch(
		ctx,
		platformApp,
		client.MergeFrom(statusBeforeChange),
	); err != nil {
		return false, fmt.Errorf("patch PlatformApp status: %w", err)
	}

	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlatformAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.PlatformApp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("platformapp").
		Complete(r)
}
