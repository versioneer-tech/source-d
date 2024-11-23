package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	packageralphav1 "github.com/versioneer-tech/source-d/api/alphav1"
)

var _ = Describe("Source Controller", func() {
	Context("When reconciling a Source resource", func() {
		const (
			resourceName = "test-resource"
			resourceNS   = "default"
			secretName   = "test-secret"
			bucketName   = "test-bucket"
			awsRegion    = "eu-central-1"
			endpointURL  = "https://s3.test"
			accessKey    = "test-access-key"
			secretKey    = "test-secret-key"
			expectedSize = "1Mi"
			storageClass = "rclone"
		)

		var ctx = context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNS,
		}

		BeforeEach(func() {
			By("creating the referenced Secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: resourceNS,
				},
				Data: map[string][]byte{
					"AWS_ACCESS_KEY_ID":     []byte(accessKey),
					"AWS_SECRET_ACCESS_KEY": []byte(secretKey),
					"AWS_ENDPOINT_URL":      []byte(endpointURL),
					"AWS_REGION":            []byte(awsRegion),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating the custom resource for the Kind Source")
			source := &packageralphav1.Source{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNS,
				},
				Spec: packageralphav1.SourceSpec{
					Access: packageralphav1.Access{
						SecretName: secretName,
						BucketName: bucketName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, source)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the created resources")
			source := &packageralphav1.Source{}
			err := k8sClient.Get(ctx, typeNamespacedName, source)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, source)).To(Succeed())

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: resourceNS}, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			pv := &corev1.PersistentVolume{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName}, pv)
			if !errors.IsNotFound(err) {
				Expect(k8sClient.Delete(ctx, pv)).To(Succeed())
			}

			pvc := &corev1.PersistentVolumeClaim{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: resourceNS}, pvc)
			if !errors.IsNotFound(err) {
				Expect(k8sClient.Delete(ctx, pvc)).To(Succeed())
			}
		})

		It("should create PersistentVolume and PersistentVolumeClaim", func() {
			By("reconciling the created Source resource")
			controllerReconciler := &SourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("verifying the PersistentVolume was created")
			pv := &corev1.PersistentVolume{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName}, pv)).To(Succeed())
			Expect(pv.Spec.StorageClassName).To(Equal(storageClass))
			Expect(pv.Spec.PersistentVolumeSource.CSI).NotTo(BeNil())
			Expect(pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["remote"]).To(Equal("s3"))
			Expect(pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["s3-endpoint"]).To(Equal(endpointURL))
			Expect(pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["s3-region"]).To(Equal(awsRegion))

			By("verifying the PersistentVolumeClaim was created")
			pvc := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: resourceNS}, pvc)).To(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse(expectedSize)))
			Expect(*pvc.Spec.StorageClassName).To(Equal(storageClass))
		})
	})
})
