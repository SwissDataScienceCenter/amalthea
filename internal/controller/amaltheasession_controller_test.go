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
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	// "runtime/debug"
)

func newReconciler() *AmaltheaSessionReconciler {
	return &AmaltheaSessionReconciler{
		Client:        k8sClient,
		Scheme:        k8sClient.Scheme(),
		MetricsClient: k8sMetricsClient,
	}
}

var foregroundDelete metav1.DeletionPropagation = metav1.DeletePropagationForeground
var deleteOptions *client.DeleteOptions = &client.DeleteOptions{PropagationPolicy: &foregroundDelete}

const namespace string = "default"

func getRandomName() string {
	prefix := "amalthea-test-"
	const length int = 8
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return prefix + string(result)
}

var _ = Describe("AmaltheaSession Controller", func() {
	Context("When reconciling a resource", func() {
		var resourceName string
		var typeNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession

		BeforeEach(func(ctx SpecContext) {
			By("creating the custom resource for the Kind AmaltheaSession")
			resourceName = getRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
			}
			err := k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
			if err != nil && errors.IsNotFound(err) {
				resource := &amaltheadevv1alpha1.AmaltheaSession{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
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

		AfterEach(func(ctx SpecContext) {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			By("Cleanup the specific resource instance AmaltheaSession")
			Expect(k8sClient.Delete(ctx, amaltheasession, deleteOptions)).To(Succeed())
		})

		It("should successfully reconcile the resource", func(ctx SpecContext) {
			By("Reconciling the created resource")
			controllerReconciler := newReconciler()
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When reconciling a resource without ingress", func() {
		var resourceName string
		var typeNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession

		BeforeEach(func(ctx SpecContext) {
			By("creating the custom resource for the Kind AmaltheaSession")
			resourceName = getRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
			}
			err := k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
			if err != nil && errors.IsNotFound(err) {
				resource := &amaltheadevv1alpha1.AmaltheaSession{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
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

		AfterEach(func(ctx SpecContext) {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &amaltheadevv1alpha1.AmaltheaSession{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AmaltheaSession")
			Expect(k8sClient.Delete(ctx, resource, deleteOptions)).To(Succeed())
		})

		It("should successfully reconcile the resource", func(ctx SpecContext) {
			By("Reconciling the created resource")
			controllerReconciler := newReconciler()
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("Adopting secrets", func() {
		var resourceName string
		var secretName string
		var typeNamespacedName types.NamespacedName
		var secretNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession
		var secret *corev1.Secret

		BeforeEach(func(ctx SpecContext) {
			resourceName = getRandomName()
			secretName = getRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
			}
			secretNamespacedName = types.NamespacedName{
				Name:      secretName,
				Namespace: namespace,
			}
			secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespacedName.Namespace}}
			tlsSecretName := secretName

			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image: "debian:bookworm-slim",
						Port:  8000,
					},
					Ingress: &amaltheadevv1alpha1.Ingress{
						Host:      "test.com",
						TLSSecret: &amaltheadevv1alpha1.SessionSecretRef{Name: tlsSecretName},
					},
				},
			}

			err := k8sClient.Get(ctx, secretNamespacedName, secret)
			if err != nil && errors.IsNotFound(err) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}
		})

		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, secret, deleteOptions)).To(Succeed())
			Expect(k8sClient.Delete(ctx, amaltheasession, deleteOptions)).To(Succeed())
		})

		DescribeTable("Manage secrets",
			func(ctx SpecContext, adoptSecrets bool) {
				By("Ensuring the session has the correct configuration")
				amaltheasession.Spec.Ingress.TLSSecret.Adopt = adoptSecrets
				err := k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
				if err != nil && errors.IsNotFound(err) {
					Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
				}

				actual := amaltheadevv1alpha1.AmaltheaSession{
					ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
				}
				Expect(k8sClient.Get(ctx, typeNamespacedName, &actual)).To(Succeed())
				Expect(actual.Spec.Ingress.TLSSecret.Adopt).To(Equal(adoptSecrets))

				By("Deleting the session")
				Expect(k8sClient.Delete(ctx, &actual, deleteOptions)).To(Succeed())

				By("Checking the secret existence matches expectation")
				err = k8sClient.Get(ctx, secretNamespacedName, secret)
				if adoptSecrets {
					Expect(errors.IsNotFound(err)).To(BeTrue())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("When secrets are adopted", true),
			Entry("When secrets are not adopted", false),
		)
	})

	Context("Handling SHM", func() {
		var resourceName string
		var typeNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession
		const shmSize = "1Mi"

		BeforeEach(func(ctx SpecContext) {
			resourceName = getRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image: "debian:bookworm-slim",
						Port:  8000,
					},
				},
			}
		})

		AfterEach(func(ctx SpecContext) {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
			}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AmaltheaSession")
			Expect(k8sClient.Delete(ctx, resource, deleteOptions)).To(Succeed())
		})

		DescribeTable("Manage SHM",
			func(ctx SpecContext, hasSHM bool) {
				By("Ensuring the StatefulSet contains SHM accordingly")
				if hasSHM {
					quantity, err := resource.ParseQuantity(shmSize)
					Expect(err).ToNot(HaveOccurred())
					amaltheasession.Spec.Session.ShmSize = &quantity
				}

				Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())

				controllerReconciler := newReconciler()
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

	Context("When testing hibernation", func() {
		var resourceName string
		var typeNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession

		BeforeEach(func(ctx SpecContext) {
			By("creating the custom resource for the Kind AmaltheaSession")
			resourceName = getRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image:   "debian:bookworm-slim",
						Command: []string{"sleep"},
						Args:    []string{"3600"},
						Port:    8000,
					},
					Culling: amaltheadevv1alpha1.Culling{
						MaxAge: metav1.Duration{
							Duration: 10 * time.Minute,
						},
						MaxIdleDuration: metav1.Duration{
							Duration: 10 * time.Second,
						},
						MaxStartingDuration: metav1.Duration{
							Duration: 2 * time.Minute,
						},
						MaxFailedDuration: metav1.Duration{
							Duration: 5 * time.Minute,
						},
						MaxHibernatedDuration: metav1.Duration{
							Duration: 15 * time.Second,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			err := k8sClient.Delete(ctx, amaltheasession, deleteOptions)
			Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		})

		It("should successfully delete hibernated resources", func(ctx SpecContext) {
			By("Checking if the custom resource was successfully created")
			controllerReconciler := newReconciler()
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				found := &amaltheadevv1alpha1.AmaltheaSession{}
				return k8sClient.Get(ctx, typeNamespacedName, found)
			}, time.Minute, time.Second).WithContext(ctx).Should(Succeed())

			By("Checking if StatefulSet was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.StatefulSet{}
				return k8sClient.Get(ctx, typeNamespacedName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Marking the session as hibernated")
			actual := amaltheadevv1alpha1.AmaltheaSession{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, &actual)).To(Succeed())

			actual.Spec.Hibernated = true
			Expect(controllerReconciler.Update(ctx, &actual)).To(Succeed())

			By("Checking if the custom resource was successfully automatically deleted")
			Eventually(func() bool {
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				found := &amaltheadevv1alpha1.AmaltheaSession{}
				err := k8sClient.Get(ctx, typeNamespacedName, found)
				return errors.IsNotFound(err)
			}, time.Minute, time.Second).WithContext(ctx).Should(BeTrue())
		})

		It("should clear statuses after hibernation", func(ctx SpecContext) {
			By("Checking if the custom resource was successfully created")
			controllerReconciler := newReconciler()
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())

			By("Artificially adding statuses")
			newTimestamp := metav1.NewTime(time.Now().Round(time.Second))
			amaltheasession.Status.FailingSince = newTimestamp
			amaltheasession.Status.IdleSince = newTimestamp
			Expect(controllerReconciler.Status().Update(ctx, amaltheasession)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
			Expect(amaltheasession.Status.FailingSince).To(Equal(newTimestamp))
			Expect(amaltheasession.Status.IdleSince).To(Equal(newTimestamp))

			By("Marking the session as hibernated")
			amaltheasession.Spec.Hibernated = true
			Expect(controllerReconciler.Update(ctx, amaltheasession)).To(Succeed())

			By("Checking that the appropriate timestamps in the status have been reset")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
			Expect(amaltheasession.Status.HibernatedSince).ToNot(BeZero())
			Expect(amaltheasession.Status.IdleSince).To(BeZero())
			Expect(amaltheasession.Status.FailingSince).To(BeZero())
			Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.Hibernated))
		})

		It("should clear statuses after resuming", func(ctx SpecContext) {
			By("Checking if the custom resource was successfully created")
			controllerReconciler := newReconciler()
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())

			By("Marking the session as hibernated")
			amaltheasession.Spec.Hibernated = true
			Expect(controllerReconciler.Update(ctx, amaltheasession)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())

			By("Artificially adding statuses")
			newTimestamp := metav1.NewTime(time.Now().Round(time.Second))
			amaltheasession.Status.FailingSince = newTimestamp
			amaltheasession.Status.IdleSince = newTimestamp
			Expect(controllerReconciler.Status().Update(ctx, amaltheasession)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
			Expect(amaltheasession.Status.HibernatedSince).ToNot(BeZero())
			Expect(amaltheasession.Status.FailingSince).To(Equal(newTimestamp))
			Expect(amaltheasession.Status.IdleSince).To(Equal(newTimestamp))

			By("Resuming the session")
			amaltheasession.Spec.Hibernated = false
			Expect(controllerReconciler.Update(ctx, amaltheasession)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the appropriate timestamps in the status have been reset")
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
			Expect(amaltheasession.Status.HibernatedSince).To(BeZero())
			Expect(amaltheasession.Status.IdleSince).To(BeZero())
			Expect(amaltheasession.Status.FailingSince).To(BeZero())
			Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.NotReady))
		})
	})
})
