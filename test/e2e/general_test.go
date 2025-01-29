package e2e

import (
	"context"
	"os/exec"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/test/utils"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("reconcile strategies", Ordered, func() {
	const namespace = "amalthea"
	var k8sClient client.Client
	var mgr manager.Manager
	var stopController context.CancelFunc

	BeforeAll(func(ctx SpecContext) {
		var ctrlCtx context.Context
		ctrlCtx, stopController = context.WithCancel(context.Background())
		utils.CreateNamespace(namespace)
		By("installing CRDs")
		cmd := exec.Command("make", "install")
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		// NOTE: If this sleep is removed then the controller cannot find the CRD in the cluster
		time.Sleep(time.Second * 5)
		mgr, err = utils.GetController(namespace)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		_, testConf := GinkgoConfiguration()
		if testConf.VeryVerbose {
			ctrl.SetLogger(GinkgoLogr.WithName("operator"))
		} else {
			ctrl.SetLogger(logr.Discard())
		}
		By("starting controller")
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctrlCtx)).To(Succeed())
		}()
		k8sClient, err = utils.GetK8sClient(ctx, namespace)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	AfterAll(func(ctx SpecContext) {
		By("deleting all resources")
		Expect(
			k8sClient.DeleteAllOf(ctx, &amaltheadevv1alpha1.AmaltheaSession{}, client.InNamespace(namespace)),
		).To(Succeed())
		By("stopping controller")
		stopController()
		By("uninstalling CRDs")
		cmd := exec.Command("make", "uninstall")
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	Context("using reconcile strategies", func() {
		var resourceName string
		var typeNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession

		BeforeEach(func(ctx SpecContext) {
			By("creating the custom resource for the Kind AmaltheaSession")
			resourceName = utils.GetRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image:          "debian:bookworm-slim",
						Command:        []string{"sleep"},
						Args:           []string{"3600"},
						ReadinessProbe: amaltheadevv1alpha1.ReadinessProbe{Type: amaltheadevv1alpha1.None},
					},
				},
			}
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &amaltheadevv1alpha1.AmaltheaSession{}
				return k8sClient.Get(ctx, typeNamespacedName, found)
			}, time.Minute, time.Second).WithContext(ctx).Should(Succeed())
			By("Checking if the session is running")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.Running))
			}).WithContext(ctx).WithTimeout(time.Minute * 2).Should(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, amaltheasession)).To(Succeed())
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
			}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())
			patched := amaltheasession.DeepCopy()
			By("Patching the session")
			newMemory := resource.MustParse("100Mi")
			patched.Spec.Session.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: newMemory}
			Expect(k8sClient.Update(ctx, patched)).To(Succeed())
			By("Checking the session was restarted")
			Eventually(func(g Gomega) {
				sessionPod, err = patched.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Spec.Containers[0].Resources.Requests.Memory()).To(Equal(&newMemory))
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(sessionPod.GetUID()).To(Not(Equal(initialUID)))
			}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())
		})

		It("should not restart the session when strategy is never", func(ctx SpecContext) {
			By("Making strategy never")
			patched := amaltheasession.DeepCopy()
			patched.Spec.ReconcileStrategy = amaltheadevv1alpha1.Never
			Expect(k8sClient.Update(ctx, patched)).Should(Succeed())
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
			}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())
			patched = amaltheasession.DeepCopy()
			By("Patching the session")
			newMemory := resource.MustParse("100Mi")
			patched.Spec.Session.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: newMemory}
			Expect(k8sClient.Update(ctx, patched)).To(Succeed())
			By("Checking the session is not restarted")
			Consistently(func(g Gomega) {
				sessionPod, err = patched.Pod(ctx, k8sClient)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sessionPod).NotTo(BeNil())
				g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
				g.Expect(sessionPod.GetUID()).To(Equal(initialUID))
			}, "30s").WithContext(ctx).Should(Succeed())
		})

		It(
			"should apply the changes only after hibernating and resuming when the strategy is whenFailedOrHibernated",
			func(ctx SpecContext,
			) {
				By("Making strategy whenFailedOrHibernated")
				patched := amaltheasession.DeepCopy()
				patched.Spec.ReconcileStrategy = amaltheadevv1alpha1.WhenFailedOrHibernated
				Expect(k8sClient.Update(ctx, patched)).To(Succeed())
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
				}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())
				patched = amaltheasession.DeepCopy()
				By("Patching the session")
				newMemory := resource.MustParse("100Mi")
				patched.Spec.Session.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: newMemory}
				Expect(k8sClient.Update(ctx, patched)).To(Succeed())
				By("Checking the session is not restarted or changed")
				Consistently(func(g Gomega) {
					sessionPod, err = patched.Pod(ctx, k8sClient)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(sessionPod).NotTo(BeNil())
					g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
					g.Expect(sessionPod.GetUID()).To(Equal(initialUID))
					g.Expect(sessionPod.Spec.Containers[0].Resources.Requests.Memory()).ShouldNot(Equal(&newMemory))
				}, "30s").WithContext(ctx).Should(Succeed())
				By("Hibernating the session")
				patched = &amaltheadevv1alpha1.AmaltheaSession{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, patched)).To(Succeed())
				patched.Spec.Hibernated = true
				Expect(k8sClient.Update(ctx, patched)).To(Succeed())
				By("Resuming the session we should see the new changes")
				patched = &amaltheadevv1alpha1.AmaltheaSession{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, patched)).To(Succeed())
				patched.Spec.Hibernated = false
				Expect(k8sClient.Update(ctx, patched)).To(Succeed())
				Eventually(func(g Gomega) {
					sessionPod, err = amaltheasession.Pod(ctx, k8sClient)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(sessionPod).NotTo(BeNil())
					g.Expect(sessionPod.Status.Phase).To(Equal(corev1.PodRunning))
					g.Expect(sessionPod.Spec.Containers[0].Resources.Requests.Memory()).To(Equal(&newMemory))
				}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())
			})
	})

	Context("When the session is failing", func() {
		var resourceName string
		var typeNamespacedName types.NamespacedName
		var amaltheasession *amaltheadevv1alpha1.AmaltheaSession

		BeforeEach(func(ctx SpecContext) {
			By("creating the custom resource for the Kind AmaltheaSession")
			resourceName = utils.GetRandomName()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			amaltheasession = &amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: metav1.ObjectMeta{Name: typeNamespacedName.Name, Namespace: typeNamespacedName.Namespace},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image:          "debian:bookworm-slim",
						Command:        []string{"sleep"},
						Args:           []string{"3600"},
						ReadinessProbe: amaltheadevv1alpha1.ReadinessProbe{Type: amaltheadevv1alpha1.None},
					},
				},
			}
		})

		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, amaltheasession)).To(Succeed())
		})

		It("should indicate the reason in the status when the image does not exist", func(ctx SpecContext) {
			amaltheasession.Spec.Session.Image = "renku/not-existing-image"
			By("Checking if the custom resource was successfully created")
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Eventually the status should be failed and contain the error")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.Failed))
				g.Expect(amaltheasession.Status.Error).To(ContainSubstring("failure to retrieve image for container"))
			}).WithContext(ctx).WithTimeout(time.Minute * 2).Should(Succeed())
		})

		It("should indicate the reason when the executable does not exist", func(ctx SpecContext) {
			amaltheasession.Spec.Session.Command = []string{"does-not-exist"}
			By("Checking if the custom resource was successfully created")
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Eventually the status should be failed and contain the error")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.Failed))
				g.Expect(amaltheasession.Status.Error).To(ContainSubstring("executable"))
			}).WithContext(ctx).WithTimeout(time.Minute * 2).Should(Succeed())
		})

		It("should indicate the reason in the status when the disk runs out of space", func(ctx SpecContext) {
			storageSize := resource.MustParse("1G")
			amaltheasession.Spec.Session.Storage.Size = &storageSize
			amaltheasession.Spec.ExtraInitContainers = []corev1.Container{
				{
					Name:  "test",
					Image: "debian:bookworm-slim",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "amalthea-volume",
							MountPath: "/test",
						},
					},
					Command: []string{"head", "-c", "5G", "</dev/urandom", ">/test/myfile"},
				},
			}
			By("Checking if the custom resource was successfully created")
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Eventually the status should be failed and contain the error")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.Failed))
				g.Expect(amaltheasession.Status.Error).To(ContainSubstring("disk"))
			}).WithContext(ctx).WithTimeout(time.Minute * 3).Should(Succeed())
		})

		It("should not fail when it is not schedulable", func(ctx SpecContext) {
			amaltheasession.Spec.Session.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{"cpu": resource.MustParse("10000")},
			}
			By("Checking if the custom resource was successfully created")
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Eventually the status should be failed and contain the error")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.NotReady))
			}, "30s").WithContext(ctx).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.NotReady))
				g.Expect(amaltheasession.Status.Error).To(ContainSubstring("more resources than available"))
			}).WithContext(ctx).WithTimeout(time.Minute * 3).Should(Succeed())
		})
	})
})
