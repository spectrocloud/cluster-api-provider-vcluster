package v1alpha1

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// log is for logging in this package.
var vclusterlog = logf.Log.WithName("vcluster-resource")

func (r *VCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-vcluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=vclusters,verbs=create;update,versions=v1alpha1,name=mvcluster.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-vcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=vclusters,verbs=create;update;delete,versions=v1alpha1,name=vvcluster.kb.io,admissionReviewVersions=v1

var (
	_ admission.CustomDefaulter = &VCluster{}
	_ admission.CustomValidator = &VCluster{}
)

// Default implements admission.CustomDefaulter so a webhook will be registered for the type
func (r *VCluster) Default(ctx context.Context, obj runtime.Object) error {
	vclusterlog.Info("default", "name", r.Name)
	return nil
}

// ValidateCreate implements admission.CustomValidator so a webhook will be registered for the type
func (r *VCluster) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	vclusterlog.Info("validate create", "name", r.Name)
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator so a webhook will be registered for the type
func (r *VCluster) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	vclusterlog.Info("validate update", "name", r.Name)
	oldVcluster := oldObj.(*VCluster)
	newVcluster := newObj.(*VCluster)

	if newVcluster.Name != oldVcluster.Name {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("vcluster name change is not allowed, old=%s, new=%s", oldVcluster.Name, newVcluster.Name))
	}

	return nil, nil
}

// ValidateDelete implements admission.CustomValidator so a webhook will be registered for the type
func (r *VCluster) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	vclusterlog.Info("validate delete", "name", r.Name)
	return nil, nil
}
