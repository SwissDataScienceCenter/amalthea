package e2e

import (
	"log"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	amaltheadevv1alpha1 "github.com/SwissDataScienceCenter/amalthea/api/v1alpha1"
	"github.com/SwissDataScienceCenter/amalthea/test/utils"
)

var _ = Describe("controller", Ordered, func() {
	const helmChart = "helm-chart/amalthea-sessions"
	const release = "amalthea"
	const namespace = "amalthea"
	var k8sClient client.Client

	BeforeAll(func(ctx SpecContext) {
		By("installing amalthea session helm chart")
		Expect(utils.InstallHelmChart(ctx, namespace, release, helmChart)).To(Succeed())
		clnt, err := utils.GetK8sClient(ctx, namespace)
		Expect(err).NotTo(HaveOccurred())
		k8sClient = clnt
	})

	AfterAll(func(ctx SpecContext) {
		Expect(k8sClient.DeleteAllOf(ctx, &amaltheadevv1alpha1.AmaltheaSession{}, client.InNamespace(namespace))).To(Succeed())
		Expect(utils.UninstallHelmChart(ctx, namespace, release)).To(Succeed())
	})

	Context("operator from helm chart", func() {
		It("should run a simple session successfully", func(ctx SpecContext) {
			session := amaltheadevv1alpha1.AmaltheaSession{
				ObjectMeta: v1.ObjectMeta{Name: "test1", Namespace: namespace},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image:   "debian:bookworm-slim",
						Command: []string{"sleep", "infinity"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &session)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: session.Name, Namespace: session.Namespace}, &session)).To(Succeed())
				g.Expect(session.Status.State).To(Equal(amaltheadevv1alpha1.Running))
			}).WithContext(ctx).WithPolling(time.Second * 2).WithTimeout(time.Minute * 2).Should(Succeed())
		})
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
						Image:   "debian:bookworm-slim",
						Command: []string{"sleep"},
						Args:    []string{"3600"},
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

		It("should apply the changes only after hibernating and resuming when the strategy is whenFailedOrHibernated", func(ctx SpecContext) {
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
			}, "30s").WithContext(ctx)
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

	FContext("When the session is failing", func() {
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
						Image:   "debian:bookworm-slim",
						Command: []string{"sleep"},
						Args:    []string{"3600"},
					},
				},
			}
		})

		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, amaltheasession)).To(Succeed())
		})

		It("should indicate the reason in the status when the image does not exist", func(ctx SpecContext) {
			amaltheasession.Spec.Session.Image = "renku/not-existing-image"
			Expect(k8sClient.Create(ctx, amaltheasession)).To(Succeed())
			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &amaltheadevv1alpha1.AmaltheaSession{}
				return k8sClient.Get(ctx, typeNamespacedName, found)
			}, time.Minute, time.Second).WithContext(ctx).Should(Succeed())
			By("Eventually the status should be failed and contain the error")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, amaltheasession)).To(Succeed())
				g.Expect(amaltheasession.Status.State).To(Equal(amaltheadevv1alpha1.Failed))
				g.Expect(amaltheasession.Status.Error).To(ContainSubstring("the image"))
				g.Expect(amaltheasession.Status.Error).To(ContainSubstring("cannot be found"))
				log.Printf("status %+v", amaltheasession.Status)
			}).WithContext(ctx).WithTimeout(time.Minute * 2).Should(Succeed())
		})

		// It("should indicate the reason in the status when the disk runs out of space" func() {
		//
		// })
	})
})
