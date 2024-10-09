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
						Sidecars: amaltheadevv1alpha1.Sidecars{
							Image: "renku/sidecars:0.0.1",
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
						Sidecars: amaltheadevv1alpha1.Sidecars{
							Image: "renku/sidecars:0.0.1",
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
					Sidecars: amaltheadevv1alpha1.Sidecars{
						Image: "renku/sidecars:0.0.1",
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
					Sidecars: amaltheadevv1alpha1.Sidecars{
						Image: "renku/sidecars:0.0.1",
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
					Expect(err).To(BeNil())
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
	})

	Context("When using reconcile strategies", func() {
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
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image:   "debian:bookworm-slim",
						Command: []string{"sleep"},
						Args:    []string{"3600"},
						Port:    8000,
					},
				},
			}
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &amaltheadevv1alpha1.AmaltheaSession{}
				return k8sClient.Get(ctx, typeNamespacedName, found)
			}, time.Minute, time.Second).WithContext(ctx).Should(Succeed())
			By("Checking if the custom resource was successfully reconciled")
			controllerReconciler := newReconciler()
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				sessionPod, err := amaltheasession.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
			}).WithContext(ctx)
			err = k8sClient.Get(ctx, typeNamespacedName, amaltheasession)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, amaltheasession, deleteOptions)).To(Succeed())
		})

		It("should restart the session when strategy is always", func(ctx SpecContext) {
			By("Checking the strategy is always")
			Expect(amaltheasession.Spec.ReconcileStrategy).To(Equal(amaltheadevv1alpha1.Always))
			var err error
			var sessionPod *corev1.Pod
			var initialUID types.UID
			Eventually(func(g Gomega) {
				sessionPod, err = amaltheasession.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				initialUID = sessionPod.GetUID()
			}).WithContext(ctx)
			patched := amaltheasession.DeepCopy()
			By("Patching the session")
			newMemory := resource.MustParse("100Mi")
			patched.Spec.Session.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: newMemory}
			err = k8sClient.Update(ctx, patched)
			Expect(err).NotTo(HaveOccurred())
			controllerReconciler := newReconciler()
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			By("Checking the session was restarted")
			Eventually(func(g Gomega) {
				sessionPod, err = patched.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Spec.Containers[0].Resources.Requests.Memory()).To(Equal(newMemory))
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(sessionPod.GetUID()).To(Not(Equal(initialUID)))
			}).WithContext(ctx)
		})

		It("should not restart the session when strategy is never", func(ctx SpecContext) {
			controllerReconciler := newReconciler()
			By("Making strategy never")
			patched := amaltheasession.DeepCopy()
			patched.Spec.ReconcileStrategy = amaltheadevv1alpha1.Never
			Expect(k8sClient.Update(ctx, patched)).Should(Succeed())
			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})).Error().ShouldNot(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
			Expect(amaltheasession.Spec.ReconcileStrategy).To(Equal(amaltheadevv1alpha1.Never))
			var err error
			var sessionPod *corev1.Pod
			var initialUID types.UID
			Eventually(func(g Gomega) {
				sessionPod, err = amaltheasession.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				initialUID = sessionPod.GetUID()
			}).WithContext(ctx)
			patched = amaltheasession.DeepCopy()
			By("Patching the session")
			newMemory := resource.MustParse("100Mi")
			patched.Spec.Session.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: newMemory}
			err = k8sClient.Update(ctx, patched)
			Expect(err).NotTo(HaveOccurred())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			By("Checking the session is not restarted")
			Consistently(func(g Gomega) {
				sessionPod, err = patched.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(sessionPod.GetUID()).To(Equal(initialUID))
			}, "30s").WithContext(ctx)
		})

		It("should apply the changes only after hibernating and resuming when the strategy is whenFailedOrHibernated", func(ctx SpecContext) {
			controllerReconciler := newReconciler()
			By("Making strategy whenFailedOrHibernated")
			patched := amaltheasession.DeepCopy()
			patched.Spec.ReconcileStrategy = amaltheadevv1alpha1.WhenFailedOrHibernated
			Expect(k8sClient.Update(ctx, patched)).Should(Succeed())
			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})).Error().ShouldNot(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
			Expect(amaltheasession.Spec.ReconcileStrategy).To(Equal(amaltheadevv1alpha1.WhenFailedOrHibernated))
			var err error
			var sessionPod *corev1.Pod
			var initialUID types.UID
			Eventually(func(g Gomega) {
				sessionPod, err = amaltheasession.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				initialUID = sessionPod.GetUID()
			}).WithContext(ctx)
			patched = amaltheasession.DeepCopy()
			By("Patching the session")
			newMemory := resource.MustParse("100Mi")
			patched.Spec.Session.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: newMemory}
			err = k8sClient.Update(ctx, patched)
			Expect(err).NotTo(HaveOccurred())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			By("Checking the session is not restarted or changed")
			Consistently(func(g Gomega) {
				sessionPod, err = patched.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(sessionPod.GetUID()).To(Equal(initialUID))
				g.Expect(sessionPod.Spec.Containers[0].Resources.Requests.Memory()).ShouldNot(Equal(&newMemory))
			}, "30s").WithContext(ctx)
			By("Hibernating the session")
			patched = &amaltheadevv1alpha1.AmaltheaSession{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, patched)).To(Succeed())
			patched.Spec.Hibernated = true
			Expect(k8sClient.Update(ctx, patched)).Should(Succeed())
			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})).Error().ShouldNot(HaveOccurred())
			By("Resuming the session we should see the new changes")
			patched = &amaltheadevv1alpha1.AmaltheaSession{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, patched)).To(Succeed())
			patched.Spec.Hibernated = false
			Expect(k8sClient.Update(ctx, patched)).Should(Succeed())
			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})).Error().ShouldNot(HaveOccurred())
			Eventually(func(g Gomega) {
				sessionPod, err = amaltheasession.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(sessionPod.Spec.Containers[0].Resources.Requests.Memory()).Should(Equal(&newMemory))
			}).WithContext(ctx)
		})
	})

})
