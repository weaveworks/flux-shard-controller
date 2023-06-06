package v1alpha1

import (
	"github.com/fluxcd/pkg/apis/meta"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ReconciliationFailedReason represents the fact that
	// the reconciliation failed.
	ReconciliationFailedReason string = "ReconciliationFailed"

	// ReconciliationSucceededReason represents the fact that
	// the reconciliation succeeded.
	ReconciliationSucceededReason string = "ReconciliationSucceeded"
)

// SetFluxShardSetReadiness sets the ready condition with the given status, reason and message.
func SetFluxShardSetReadiness(set *FluxShardSet, status metav1.ConditionStatus, reason, message string) {
	set.Status.ObservedGeneration = set.ObjectMeta.Generation
	newCondition := metav1.Condition{
		Type:    meta.ReadyCondition,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
	apimeta.SetStatusCondition(&set.Status.Conditions, newCondition)
}

// SetReadyWithInventory updates the FluxShardSet to reflect the new readiness and
// store the current inventory.
func SetReadyWithInventory(set *FluxShardSet, inventory *ResourceInventory, reason, message string) {
	set.Status.Inventory = inventory

	if len(inventory.Entries) == 0 {
		set.Status.Inventory = nil
	}

	SetFluxShardSetReadiness(set, metav1.ConditionTrue, reason, message)
}

// FluxShardSetReadiness returns the readiness condition of the FluxShardSet.
func FluxShardSetReadiness(set *FluxShardSet) metav1.ConditionStatus {
	return apimeta.FindStatusCondition(set.Status.Conditions, meta.ReadyCondition).Status
}
