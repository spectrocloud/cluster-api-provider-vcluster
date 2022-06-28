package v1alpha1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
	_ webhook.Defaulter = &VCluster{}
	_ webhook.Validator = &VCluster{}
)

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *VCluster) Default() {
	vclusterlog.Info("default", "name", r.Name)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *VCluster) ValidateCreate() error {
	vclusterlog.Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *VCluster) ValidateUpdate(old runtime.Object) error {
	vclusterlog.Info("validate update", "name", r.Name)
	oldVcluster := old.(*VCluster)

	if *r.Spec.KubernetesVersion != *oldVcluster.Spec.KubernetesVersion {
		return apierrors.NewBadRequest(fmt.Sprintf("vcluster k8s version change is not allowed, old=%s, new=%s", *oldVcluster.Spec.KubernetesVersion, *r.Spec.KubernetesVersion))
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *VCluster) ValidateDelete() error {
	vclusterlog.Info("validate delete", "name", r.Name)
	return nil
}
