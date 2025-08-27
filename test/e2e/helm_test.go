package e2e

import (
	"os/exec"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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
		utils.Run(exec.Command("make", "uninstall")) //nolint:errcheck
		ctrl.SetLogger(logr.Discard())
		By("installing amalthea session helm chart")
		Expect(utils.InstallHelmChart(ctx, namespace, release, helmChart)).To(Succeed())
		clnt, err := utils.GetK8sClient(ctx, namespace)
		Expect(err).NotTo(HaveOccurred())
		k8sClient = clnt
	})

	AfterAll(func(ctx SpecContext) {
		Expect(
			k8sClient.DeleteAllOf(ctx, &amaltheadevv1alpha1.HpcAmaltheaSession{}, client.InNamespace(namespace)),
		).To(Succeed())
		Expect(utils.UninstallHelmChart(ctx, namespace, release)).To(Succeed())
	})

	Context("operator from helm chart", func() {
		It("should run a simple session successfully", func(ctx SpecContext) {
			session := amaltheadevv1alpha1.HpcAmaltheaSession{
				ObjectMeta: v1.ObjectMeta{Name: "test1", Namespace: namespace},
				Spec: amaltheadevv1alpha1.AmaltheaSessionSpec{
					Session: amaltheadevv1alpha1.Session{
						Image:          "debian:bookworm-slim",
						Command:        []string{"sleep", "infinity"},
						ReadinessProbe: amaltheadevv1alpha1.ReadinessProbe{Type: amaltheadevv1alpha1.None},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &session)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(
					k8sClient.Get(ctx, types.NamespacedName{Name: session.Name, Namespace: session.Namespace}, &session),
				).To(Succeed())
				g.Expect(session.Status.State).To(Equal(amaltheadevv1alpha1.Running))
			}).WithContext(ctx).WithPolling(time.Second * 2).WithTimeout(time.Minute * 2).Should(Succeed())
		})
	})
})
