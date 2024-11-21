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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	packagerv1alpha1 "github.com/versioneer-tech/source-d/api/v1alpha1"
)

// SourceReconciler reconciles a Source object
type SourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=package.r,resources=sources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=package.r,resources=sources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=package.r,resources=sources/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Source object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var source packagerv1alpha1.Source
	if err := r.Get(ctx, req.NamespacedName, &source); err != nil {
		if errors.IsNotFound(err) {
			l.Info("Source %v got deleted", req)
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get Source")
		return ctrl.Result{}, err
	}

	// Create or update PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      source.Name,
			Namespace: source.Namespace, // Use source's namespace
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		// Set PVC Spec based on Source object
		pvc.Spec.StorageClassName = &source.Spec.StorageClassName
		pvc.Spec.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(source.Spec.Size),
			},
		}
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		}
		// Set controller reference to Source object
		return controllerutil.SetControllerReference(&source, pvc, r.Scheme)

	})

	if err != nil {
		l.Error(err, "Failed to create or update PVC")
		return ctrl.Result{}, err
	}

	l.Info("PVC action result", "result", result)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&packagerv1alpha1.Source{}).
		Owns(&corev1.PersistentVolumeClaim{}). // Important: Add this line to manage PVCs
		Complete(r)
}
