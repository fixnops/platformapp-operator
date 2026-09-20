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
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/fixnops/platformapp-operator/api/v1alpha1"
)

var _ = Describe("PlatformApp Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			By("creating a valid PlatformApp custom resource")

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
				return
			}

			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			resource := &appsv1alpha1.PlatformApp{}

			err := k8sClient.Get(
				ctx,
				typeNamespacedName,
				resource,
			)
			if errors.IsNotFound(err) {
				return
			}

			Expect(err).NotTo(HaveOccurred())

			By("cleaning up the PlatformApp resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile the resource and report progressing status", func() {
			By("reconciling the PlatformApp")

			controllerReconciler := &PlatformAppReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{
					NamespacedName: typeNamespacedName,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			By("fetching the updated PlatformApp status")

			updatedPlatformApp := &appsv1alpha1.PlatformApp{}
			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					updatedPlatformApp,
				),
			).To(Succeed())

			By("verifying the processed generation")

			Expect(
				updatedPlatformApp.Status.ObservedGeneration,
			).To(Equal(updatedPlatformApp.Generation))

			By("verifying that no replicas are ready in envtest")

			Expect(
				updatedPlatformApp.Status.ReadyReplicas,
			).To(Equal(int32(0)))

			By("verifying the Available condition")

			availableCondition := apimeta.FindStatusCondition(
				updatedPlatformApp.Status.Conditions,
				conditionAvailable,
			)

			Expect(availableCondition).NotTo(BeNil())
			Expect(availableCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(availableCondition.Reason).To(Equal("ReplicasNotReady"))
			Expect(availableCondition.ObservedGeneration).To(
				Equal(updatedPlatformApp.Generation),
			)

			By("verifying the Progressing condition")

			progressingCondition := apimeta.FindStatusCondition(
				updatedPlatformApp.Status.Conditions,
				conditionProgressing,
			)

			Expect(progressingCondition).NotTo(BeNil())
			Expect(progressingCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(progressingCondition.Reason).To(Equal("WaitingForReplicas"))
			Expect(progressingCondition.ObservedGeneration).To(
				Equal(updatedPlatformApp.Generation),
			)

			By("verifying the Degraded condition")

			degradedCondition := apimeta.FindStatusCondition(
				updatedPlatformApp.Status.Conditions,
				conditionDegraded,
			)

			Expect(degradedCondition).NotTo(BeNil())
			Expect(degradedCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(degradedCondition.Reason).To(
				Equal("ReconciliationSucceeded"),
			)
			Expect(degradedCondition.ObservedGeneration).To(
				Equal(updatedPlatformApp.Generation),
			)
		})
	})
})
