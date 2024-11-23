package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	packageralphav1 "github.com/versioneer-tech/source-d/api/alphav1"
)

type SourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *SourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var source packageralphav1.Source
	if err := r.Get(ctx, req.NamespacedName, &source); err != nil {
		if errors.IsNotFound(err) {
			l.Info("Source not found, possibly deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to fetch Source")
		return ctrl.Result{}, err
	}

	secret := &corev1.Secret{}
	secretName := types.NamespacedName{Namespace: source.Namespace, Name: source.Spec.Access.SecretName}
	if err := r.Get(ctx, secretName, secret); err != nil {
		if errors.IsNotFound(err) {
			l.Info("Referenced secret not found, requeuing Source for reconciliation")
			source.Status.Error = fmt.Sprintf("Secret %s not found", source.Spec.Access.SecretName)
			if updateErr := r.Status().Update(ctx, &source); updateErr != nil {
				l.Error(updateErr, "Failed to update Source status")
			}
			return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
		}
		l.Error(err, "Failed to fetch secret")
		return ctrl.Result{}, err
	}

	awsAccessKeyID := string(secret.Data["AWS_ACCESS_KEY_ID"])
	awsSecretAccessKey := string(secret.Data["AWS_SECRET_ACCESS_KEY"])
	awsEndpointURL := string(secret.Data["AWS_ENDPOINT_URL"])
	awsRegion := string(secret.Data["AWS_REGION"])

	if awsAccessKeyID == "" || awsSecretAccessKey == "" || awsEndpointURL == "" || awsRegion == "" {
		return ctrl.Result{}, fmt.Errorf("required AWS credentials missing in secret %s", secret.Name)
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   source.Name,
			Labels: map[string]string{"source-name": source.Name},
		},
	}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, pv, func() error {
		if pv.Spec.PersistentVolumeSource.CSI == nil {
			pv.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "csi-rclone",
					VolumeHandle: source.Spec.Access.BucketName,
					VolumeAttributes: map[string]string{
						"remote":               "s3",
						"remotePath":           source.Spec.Access.BucketName,
						"s3-provider":          "AWS",
						"s3-endpoint":          awsEndpointURL,
						"s3-access-key-id":     awsAccessKeyID,
						"s3-secret-access-key": awsSecretAccessKey,
						"s3-region":            awsRegion,
					},
				},
			}
		}
		pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		pv.Spec.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}
		if pv.Spec.StorageClassName == "" {
			pv.Spec.StorageClassName = "rclone"
		}
		return nil
	})
	if err != nil {
		l.Error(err, "Failed to create or update PersistentVolume")
		return ctrl.Result{}, err
	}
	l.Info(fmt.Sprintf("PersistentVolume %s %s", pv.Name, op))

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      source.Name,
			Namespace: source.Namespace,
		},
	}
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if err := controllerutil.SetControllerReference(&source, pvc, r.Scheme); err != nil {
			return err
		}
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		pvc.Spec.Resources = corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Mi"),
			},
		}
		pvc.Spec.StorageClassName = &pv.Spec.StorageClassName
		pvc.Spec.Selector = &metav1.LabelSelector{MatchLabels: pv.Labels}
		return nil
	})

	if err != nil {
		l.Error(err, "Failed to create or update PersistentVolumeClaim")
		return ctrl.Result{}, err
	}
	l.Info(fmt.Sprintf("PersistentVolumeClaim %s/%s %s", pvc.Namespace, pvc.Name, op))

	return ctrl.Result{}, nil
}

func (r *SourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&packageralphav1.Source{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
