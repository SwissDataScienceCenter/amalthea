/*
Copyright 2024.

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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
)

var _ = Describe("AmaltheaSession Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		amaltheasession := &amaltheadevv1alpha1.AmaltheaSession{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind AmaltheaSession")
			err := k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
			if err != nil && errors.IsNotFound(err) {
				resource := &amaltheadevv1alpha1.AmaltheaSession{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
						Session: amaltheadevv1alpha1.Session{
							Image: "debian:bookworm-slim",
							Port:  8000,
						},
						Ingress: &amaltheadevv1alpha1.Ingress{
							Host: "test.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &amaltheadevv1alpha1.AmaltheaSession{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AmaltheaSession")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AmaltheaSessionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When reconciling a resource without ingress", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		amaltheasession := &amaltheadevv1alpha1.AmaltheaSession{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind AmaltheaSession")
			err := k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
			if err != nil && errors.IsNotFound(err) {
				resource := &amaltheadevv1alpha1.AmaltheaSession{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
						Session: amaltheadevv1alpha1.Session{
							Image: "debian:bookworm-slim",
							Port:  8000,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &amaltheadevv1alpha1.AmaltheaSession{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AmaltheaSession")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AmaltheaSessionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("Adopting secrets", func() {
		const resourceName = "test-resource"
		const secretName = "test-secret"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		amaltheasession := &amaltheadevv1alpha1.AmaltheaSession{}

		secretNamespacedName := types.NamespacedName{
			Name:      secretName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		secret := &corev1.Secret{}

		BeforeEach(func() {
			tlsSecretName := secretName

			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image: "debian:bookworm-slim",
						Port:  8000,
					},
					Ingress: &amaltheadevv1alpha1.Ingress{
						Host:          "test.com",
						TLSSecretName: &tlsSecretName,
					},
					AdoptSecrets: false,
				},
			}

			err := k8sClient.Get(ctx, secretNamespacedName, secret)
			if err != nil && errors.IsNotFound(err) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: "default",
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})

		DescribeTable("Manage secrets",
			func(adoptSecrets bool) {
				By("Ensuring the session has the correct configuration")
				amaltheasession.Spec.AdoptSecrets = adoptSecrets
				err := k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
				}

				actual := amaltheadevv1alpha1.AmaltheaSession{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, &actual)).To(Succeed())
				Expect(actual.Spec.AdoptSecrets).To(Equal(adoptSecrets))

				By("Deleting the session")
				Expect(k8sClient.Delete(ctx, &actual)).To(Succeed())

				By("Checking the secret existence matches expectation")
				err = k8sClient.Get(ctx, secretNamespacedName, secret)
				if adoptSecrets {
					Expect(errors.IsNotFound(err))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("When secrets are adopted", true),
			Entry("When secrets are not adopted", false),
		)
	})

	Context("Handling SHM", func() {
		const resourceName = "test-resource"
		const shmSize = "1Mi"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		amaltheasession := &amaltheadevv1alpha1.AmaltheaSession{}

		BeforeEach(func() {
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image: "debian:bookworm-slim",
						Port:  8000,
					},
				},
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &amaltheadevv1alpha1.AmaltheaSession{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AmaltheaSession")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		DescribeTable("Manage SHM",
			func(hasSHM bool) {
				By("Ensuring the StatefulSet contains SHM accordingly")
				if hasSHM {
					quantity, err := resource.ParseQuantity(shmSize)
					Expect(err).To(BeNil())
					amaltheasession.Spec.Session.ShmSize = &quantity
				}

				Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())

				controllerReconciler := &AmaltheaSessionReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				sts := &appsv1.StatefulSet{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, sts)).To(Succeed())

				mainContainer := sts.Spec.Template.Spec.Containers[0]
				volumeName := "amalthea-dev-shm"

				if hasSHM {
					Expect(mainContainer.VolumeMounts).To(ContainElement(HaveField("Name", Equal(volumeName))))
				} else {
					Expect(mainContainer.VolumeMounts).ShouldNot(ContainElement(volumeName))
				}
			},
			Entry("When SHM is configured", true),
			Entry("When SHM is not configured", false),
		)
	})
})
