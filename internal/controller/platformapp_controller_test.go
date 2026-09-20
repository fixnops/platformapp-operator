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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/fixnops/platformapp-operator/api/v1alpha1"
)

var _ = Describe("PlatformApp Controller", func() {
	Context("When reconciling a PlatformApp", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		expectedLabels := map[string]string{
			"app.kubernetes.io/name":       "platformapp",
			"app.kubernetes.io/instance":   resourceName,
			"app.kubernetes.io/managed-by": "platformapp-operator",
		}

		var controllerReconciler *PlatformAppReconciler

		BeforeEach(func() {
			By("creating a valid PlatformApp")

			platformApp := &appsv1alpha1.PlatformApp{}
			err := k8sClient.Get(
				ctx,
				typeNamespacedName,
				platformApp,
			)

			if errors.IsNotFound(err) {
				resource := &appsv1alpha1.PlatformApp{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: appsv1alpha1.PlatformAppSpec{
						Image: "nginx:1.27",
						Port:  80,
					},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			controllerReconciler = &PlatformAppReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		AfterEach(func() {
			By("cleaning up resources created by the test")

			objects := []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
				},
				&appsv1alpha1.PlatformApp{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
				},
			}

			for _, object := range objects {
				err := k8sClient.Delete(ctx, object)
				if errors.IsNotFound(err) {
					continue
				}

				Expect(err).NotTo(HaveOccurred())
			}
		})

		reconcilePlatformApp := func() {
			_, err := controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{
					NamespacedName: typeNamespacedName,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		}

		It("creates the expected Deployment and Service", func() {
			By("reconciling the PlatformApp")

			reconcilePlatformApp()

			By("fetching the PlatformApp")

			platformApp := &appsv1alpha1.PlatformApp{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					platformApp,
				),
			).To(Succeed())

			By("verifying the Deployment")

			deployment := &appsv1.Deployment{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					deployment,
				),
			).To(Succeed())

			Expect(deployment.Labels).To(Equal(expectedLabels))
			Expect(deployment.Spec.Replicas).NotTo(BeNil())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))

			Expect(
				deployment.Spec.Selector.MatchLabels,
			).To(Equal(expectedLabels))

			Expect(
				deployment.Spec.Template.Labels,
			).To(Equal(expectedLabels))

			Expect(
				deployment.Spec.Template.Spec.Containers,
			).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]

			Expect(container.Name).To(Equal("application"))
			Expect(container.Image).To(Equal("nginx:1.27"))
			Expect(container.Ports).To(HaveLen(1))
			Expect(container.Ports[0].Name).To(Equal("app-port"))
			Expect(container.Ports[0].ContainerPort).To(Equal(int32(80)))
			Expect(container.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))

			Expect(
				metav1.IsControlledBy(deployment, platformApp),
			).To(BeTrue())

			By("verifying the Service")

			service := &corev1.Service{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					service,
				),
			).To(Succeed())

			Expect(service.Labels).To(Equal(expectedLabels))
			Expect(service.Spec.Selector).To(Equal(expectedLabels))
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(service.Spec.Ports).To(HaveLen(1))

			servicePort := service.Spec.Ports[0]

			Expect(servicePort.Name).To(Equal("app-port"))
			Expect(servicePort.Port).To(Equal(int32(80)))
			Expect(servicePort.TargetPort.IntVal).To(Equal(int32(80)))
			Expect(servicePort.Protocol).To(Equal(corev1.ProtocolTCP))

			Expect(
				metav1.IsControlledBy(service, platformApp),
			).To(BeTrue())

			By("verifying the initial PlatformApp status")

			Expect(
				platformApp.Status.ObservedGeneration,
			).To(Equal(platformApp.Generation))

			Expect(
				platformApp.Status.ReadyReplicas,
			).To(Equal(int32(0)))

			availableCondition := apimeta.FindStatusCondition(
				platformApp.Status.Conditions,
				conditionAvailable,
			)
			Expect(availableCondition).NotTo(BeNil())
			Expect(availableCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(availableCondition.Reason).To(Equal("ReplicasNotReady"))

			progressingCondition := apimeta.FindStatusCondition(
				platformApp.Status.Conditions,
				conditionProgressing,
			)
			Expect(progressingCondition).NotTo(BeNil())
			Expect(progressingCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(progressingCondition.Reason).To(Equal("WaitingForReplicas"))

			degradedCondition := apimeta.FindStatusCondition(
				platformApp.Status.Conditions,
				conditionDegraded,
			)
			Expect(degradedCondition).NotTo(BeNil())
			Expect(degradedCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(degradedCondition.Reason).To(
				Equal("ReconciliationSucceeded"),
			)
		})

		It("updates the Deployment when the PlatformApp spec changes", func() {
			By("performing the initial reconciliation")

			reconcilePlatformApp()

			By("changing the requested image and replicas")

			platformApp := &appsv1alpha1.PlatformApp{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					platformApp,
				),
			).To(Succeed())

			previousGeneration := platformApp.Generation
			updatedReplicas := int32(3)

			platformApp.Spec.Image = "nginx:1.28"
			platformApp.Spec.Replicas = &updatedReplicas

			Expect(k8sClient.Update(ctx, platformApp)).To(Succeed())
			Expect(platformApp.Generation).To(
				BeNumerically(">", previousGeneration),
			)

			By("reconciling the updated PlatformApp")

			reconcilePlatformApp()

			By("verifying the updated Deployment")

			deployment := &appsv1.Deployment{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					deployment,
				),
			).To(Succeed())

			Expect(deployment.Spec.Replicas).NotTo(BeNil())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
			Expect(
				deployment.Spec.Template.Spec.Containers[0].Image,
			).To(Equal("nginx:1.28"))

			By("verifying status processed the new generation")

			updatedPlatformApp := &appsv1alpha1.PlatformApp{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					updatedPlatformApp,
				),
			).To(Succeed())

			Expect(
				updatedPlatformApp.Status.ObservedGeneration,
			).To(Equal(updatedPlatformApp.Generation))

			Expect(
				updatedPlatformApp.Status.ReadyReplicas,
			).To(Equal(int32(0)))

			progressingCondition := apimeta.FindStatusCondition(
				updatedPlatformApp.Status.Conditions,
				conditionProgressing,
			)
			Expect(progressingCondition).NotTo(BeNil())
			Expect(progressingCondition.Status).To(Equal(metav1.ConditionTrue))
		})

		It("does not update stable resources during repeated reconciliation", func() {
			By("performing the initial reconciliation")

			reconcilePlatformApp()

			By("recording the current resource versions")

			platformAppBefore := &appsv1alpha1.PlatformApp{}
			deploymentBefore := &appsv1.Deployment{}
			serviceBefore := &corev1.Service{}

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					platformAppBefore,
				),
			).To(Succeed())

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					deploymentBefore,
				),
			).To(Succeed())

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					serviceBefore,
				),
			).To(Succeed())

			platformAppResourceVersion :=
				platformAppBefore.ResourceVersion
			deploymentResourceVersion :=
				deploymentBefore.ResourceVersion
			serviceResourceVersion :=
				serviceBefore.ResourceVersion

			By("reconciling again without changing desired state")

			reconcilePlatformApp()

			By("fetching the resources after the second reconciliation")

			platformAppAfter := &appsv1alpha1.PlatformApp{}
			deploymentAfter := &appsv1.Deployment{}
			serviceAfter := &corev1.Service{}

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					platformAppAfter,
				),
			).To(Succeed())

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					deploymentAfter,
				),
			).To(Succeed())

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					serviceAfter,
				),
			).To(Succeed())

			Expect(platformAppAfter.ResourceVersion).To(
				Equal(platformAppResourceVersion),
			)

			Expect(deploymentAfter.ResourceVersion).To(
				Equal(deploymentResourceVersion),
			)

			Expect(serviceAfter.ResourceVersion).To(
				Equal(serviceResourceVersion),
			)
		})
	})
})
