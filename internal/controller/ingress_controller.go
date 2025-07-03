/*
Copyright 2025.

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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	coreDNSConfigMapName      = "coredns"
	coreDNSConfigMapNamespace    = "kube-system"
	corefileKey                  = "Corefile"
	rewriteRuleFormat            = "rewrite name %s %s\n"
	managedRulesBeginMarker      = "# BEGIN IngressReconciler managed rules"
	managedRulesEndMarker        = "# END IngressReconciler managed rules"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Log                          logr.Logger
	Scheme                       *runtime.Scheme
	IngressAnnotation            string
	IngressControllerServiceName string
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Ingress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ingress", req.NamespacedName)

	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Ingress resource not found. Ignoring since object must be deleted.")
			// Trigger a reconciliation of all ingresses to remove stale rules
			return ctrl.Result{}, r.updateCoreDNSConfigMap(ctx)
		}
		log.Error(err, "unable to fetch Ingress")
		return ctrl.Result{}, err
	}

	// Filter based on annotation
	if r.IngressAnnotation != "" {
		annotations := ingress.GetAnnotations()
		if _, ok := annotations[r.IngressAnnotation]; !ok {
			log.Info("Ingress does not have the required annotation, skipping", "annotation", r.IngressAnnotation)
			// Ensure no stale rules exist for this ingress if the annotation was removed
			return ctrl.Result{}, r.updateCoreDNSConfigMap(ctx)
		}
	}

	return ctrl.Result{}, r.updateCoreDNSConfigMap(ctx)
}

func (r *IngressReconciler) updateCoreDNSConfigMap(ctx context.Context) error {
	log := r.Log.WithName("coredns-updater")

	// Get the CoreDNS configmap
	var coreDNSConfigMap corev1.ConfigMap
	if err := r.Get(ctx, client.ObjectKey{Namespace: coreDNSConfigMapNamespace, Name: coreDNSConfigMapName}, &coreDNSConfigMap); err != nil {
		log.Error(err, "unable to fetch CoreDNS ConfigMap")
		return err
	}

	// Get all ingresses in watched namespaces
	var allIngresses networkingv1.IngressList
	if err := r.List(ctx, &allIngresses); err != nil {
		log.Error(err, "unable to list Ingresses")
		return err
	}

	// Generate rewrite rules
	var newRewriteRules strings.Builder
	for _, ingress := range allIngresses.Items {
		// Apply the same annotation filter as in the main reconcile loop
		if r.IngressAnnotation != "" {
			annotations := ingress.GetAnnotations()
			if _, ok := annotations[r.IngressAnnotation]; !ok {
				continue
			}
		}

		for _, rule := range ingress.Spec.Rules {
			if rule.Host != "" {
				newRewriteRules.WriteString(fmt.Sprintf(rewriteRuleFormat, rule.Host, r.IngressControllerServiceName))
			}
		}
	}

	// Update the Corefile
	originalCorefile := coreDNSConfigMap.Data[corefileKey]
	updatedCorefile := r.injectRewriteRules(originalCorefile, newRewriteRules.String())

	// Only update if the content has changed
	if originalCorefile == updatedCorefile {
		log.Info("CoreDNS rewrite rules are already up to date.")
		return nil
	}

	coreDNSConfigMap.Data[corefileKey] = updatedCorefile

	if err := r.Update(ctx, &coreDNSConfigMap); err != nil {
		log.Error(err, "unable to update CoreDNS ConfigMap")
		return err
	}

	log.Info("Successfully updated CoreDNS ConfigMap with new rewrite rules")
	return nil
}

// injectRewriteRules takes the current Corefile content and a string of new rewrite rules,
// and returns the modified Corefile content.
// It aims to manage a block of rewrite rules demarcated by specific begin and end markers.
func (r *IngressReconciler) injectRewriteRules(corefileContent string, newRules string) string {
	corefileLines := strings.Split(corefileContent, "\n")
	var resultBuilder strings.Builder

	// --- Part 1: Find existing managed rule block markers ---
	startIndex := -1
	endIndex := -1

	for i, line := range corefileLines {
		trimmedLine := strings.TrimSpace(line)

		// Check for combined begin and end markers on the same line
		if strings.Contains(trimmedLine, managedRulesBeginMarker) && strings.Contains(trimmedLine, managedRulesEndMarker) {
			startIndex = i
			endIndex = i
			break // Found a self-contained block
		}
		// Check for the begin marker
		if strings.Contains(trimmedLine, managedRulesBeginMarker) {
			if startIndex == -1 { // Take the first occurrence
				startIndex = i
			}
		}
		// Check for the end marker, ensuring it's after a begin marker
		if strings.Contains(trimmedLine, managedRulesEndMarker) {
			if startIndex != -1 && i >= startIndex {
				endIndex = i
				break // Found a valid end for a previously started block
			}
		}
	}

	// --- Part 2: Construct the new managed rules block ---
	var newManagedBlock strings.Builder
	newManagedBlock.WriteString(managedRulesBeginMarker)
	newManagedBlock.WriteString("\n")
	if newRules != "" {
		newManagedBlock.WriteString(strings.TrimSpace(newRules))
		newManagedBlock.WriteString("\n")
	}
	newManagedBlock.WriteString(managedRulesEndMarker)
	newManagedBlock.WriteString("\n")

	// --- Part 3: Integrate the new block into the Corefile content ---

	// Case 1: Valid markers found. Replace the content between them.
	if startIndex != -1 && endIndex != -1 && startIndex <= endIndex {
		// Append lines before the original managed block
		for i := 0; i < startIndex; i++ {
			resultBuilder.WriteString(corefileLines[i])
			resultBuilder.WriteString("\n")
		}

		// Add the new managed block
		resultBuilder.WriteString(newManagedBlock.String())

		// Append lines after the original managed block
		for i := endIndex + 1; i < len(corefileLines); i++ {
			resultBuilder.WriteString(corefileLines[i])
			resultBuilder.WriteString("\n")
		}

		// Normalize and return
		finalOutput := strings.TrimSpace(resultBuilder.String())
		if finalOutput != "" {
			return finalOutput + "\n"
		}
		return "" // Should typically not be reached if markers are involved
	}

	// Case 2: Markers not found (or invalid). Inject the new block.
	// Attempt to find an ideal insertion point (before 'kubernetes' plugin).
	insertionPoint := -1
	inServerBlockHeuristic := false // Simple heuristic to check if we are inside a server block ".:53 {}".
	for i, line := range corefileLines {
		trimmedLine := strings.TrimSpace(line)
		if strings.Contains(trimmedLine, ".:53") || strings.Contains(trimmedLine, "{") {
			inServerBlockHeuristic = true
		}
		// Prefer inserting before the 'kubernetes' plugin if found within a server block.
		if inServerBlockHeuristic && strings.Contains(line, "kubernetes") {
			insertionPoint = i
			break
		}
		// Basic way to exit server block heuristic; a proper parser would be better.
		// if strings.Contains(trimmedLine, "}") {
		//	 inServerBlockHeuristic = false
		// }
	}

	if insertionPoint != -1 {
		// Insert the new block at the determined insertion point.
		for i := 0; i < insertionPoint; i++ {
			resultBuilder.WriteString(corefileLines[i])
			resultBuilder.WriteString("\n")
		}
		resultBuilder.WriteString(newManagedBlock.String())
		for i := insertionPoint; i < len(corefileLines); i++ {
			resultBuilder.WriteString(corefileLines[i])
			resultBuilder.WriteString("\n")
		}
	} else {
		// Fallback: Append the new block to the end if no suitable insertion point was found.
		// First, write all original lines if any
		if strings.TrimSpace(corefileContent) != "" {
			resultBuilder.WriteString(strings.TrimSuffix(corefileContent, "\n")) // Avoid double newline if original ends with one
			resultBuilder.WriteString("\n")
		}
		resultBuilder.WriteString(newManagedBlock.String())
	}

	// Normalize and return
	finalOutput := strings.TrimSpace(resultBuilder.String())
	if finalOutput != "" {
		return finalOutput + "\n"
	}
	// This case handles if the original corefile was empty AND newRules were empty,
	// in which case, only the markers are added.
	return "" // Should effectively be markers if both inputs were empty.
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress").
		Complete(r)
}
