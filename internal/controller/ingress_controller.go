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

func (r *IngressReconciler) injectRewriteRules(corefile, newRules string) string {
	lines := strings.Split(corefile, "\n")
	var newCorefileContent strings.Builder
	startMarkerIndex := -1
	endMarkerIndex := -1

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.Contains(trimmedLine, managedRulesBeginMarker) && strings.Contains(trimmedLine, managedRulesEndMarker) {
			// Case where both markers are on the same line
			startMarkerIndex = i
			endMarkerIndex = i
			break
		}
		if strings.Contains(trimmedLine, managedRulesBeginMarker) {
			// If we already found a start, and this isn't a combined line, it might be a new block or error
			// For simplicity, take the first begin marker
			if startMarkerIndex == -1 {
				startMarkerIndex = i
			}
		}
		if strings.Contains(trimmedLine, managedRulesEndMarker) {
			// Only set end if start is already found and this is a valid end
			if startMarkerIndex != -1 && i >= startMarkerIndex {
				endMarkerIndex = i
				break // Found start and then end marker
			}
		}
	}

	// Case 1: Markers found - replace content between them
	if startMarkerIndex != -1 && endMarkerIndex != -1 && startMarkerIndex <= endMarkerIndex { // Allow same line for start/end
		// Append lines before the start marker
		for i := 0; i < startMarkerIndex; i++ {
			newCorefileContent.WriteString(lines[i])
			newCorefileContent.WriteString("\n")
		}

		// Add the new rules block
		newCorefileContent.WriteString(managedRulesBeginMarker)
		newCorefileContent.WriteString("\n")
		if newRules != "" {
			newCorefileContent.WriteString(strings.TrimSpace(newRules))
			newCorefileContent.WriteString("\n")
		}
		newCorefileContent.WriteString(managedRulesEndMarker)
		newCorefileContent.WriteString("\n")

		// Append lines after the end marker
		// If startMarkerIndex == endMarkerIndex, this loop should not run if endMarkerIndex+1 is out of bounds.
		// It correctly skips if the block was the last part of the file.
		for i := endMarkerIndex + 1; i < len(lines); i++ {
			newCorefileContent.WriteString(lines[i])
			newCorefileContent.WriteString("\n")
		}
	} else { // Case 2: Markers not found - try to inject or append
		insertionPoint := -1
		// Attempt to find a line containing "kubernetes" within a server block (heuristic)
		// to insert before it.
		// A more robust parser would be better, but this retains previous behavior.
		inServerBlock := false
		for i, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if strings.Contains(trimmedLine, ".:53") || strings.Contains(trimmedLine, "{") {
				// Simplistic check for start of a server block or any block
				inServerBlock = true
			}
			if inServerBlock && strings.Contains(line, "kubernetes") {
				insertionPoint = i
				break
			}
			// If exiting a block, reset inServerBlock if it's too simple a check
			// This part is tricky without a proper parser. Assuming kubernetes is not deeply nested for now.
			// if strings.Contains(trimmedLine, "}") {
			// inServerBlock = false
			// }
		}

		// Build the rules block string
		var rulesBlock strings.Builder
		rulesBlock.WriteString(managedRulesBeginMarker)
		rulesBlock.WriteString("\n")
		if newRules != "" {
			rulesBlock.WriteString(strings.TrimSpace(newRules))
			rulesBlock.WriteString("\n")
		}
		rulesBlock.WriteString(managedRulesEndMarker)
		rulesBlock.WriteString("\n")

		if insertionPoint != -1 {
			// Insert before the found kubernetes line
			for i := 0; i < insertionPoint; i++ {
				newCorefileContent.WriteString(lines[i])
				newCorefileContent.WriteString("\n")
			}
			newCorefileContent.WriteString(rulesBlock.String())
			for i := insertionPoint; i < len(lines); i++ {
				newCorefileContent.WriteString(lines[i])
				newCorefileContent.WriteString("\n")
			}
		} else {
			// Fallback: append to the end of the existing content
			// First, write all original lines
			for _, line := range lines {
				if line != "" { // Avoid adding extra newlines if original had blank lines that split would preserve
					newCorefileContent.WriteString(line)
					newCorefileContent.WriteString("\n")
				}
			}
			// Then append the new block
			// Ensure there's a newline if corefile was not empty and didn't end with one
			if corefile != "" && !strings.HasSuffix(strings.TrimSpace(corefile), "\n") && !strings.HasSuffix(newCorefileContent.String(), "\n") {
				//This check might be redundant if lines always get \n
			}
			// If corefile is empty, newCorefileContent will be empty here.
			// If corefile is not empty, it will have content.
			// The rulesBlock already ends with a newline.
			newCorefileContent.WriteString(rulesBlock.String())
		}
	}

	// Normalize output: trim whitespace and ensure single trailing newline if not empty.
	finalOutput := strings.TrimSpace(newCorefileContent.String())
	if finalOutput != "" {
		return finalOutput + "\n"
	}
	// If the corefile was empty and newRules was empty, markers are added, so it won't be ""
	// This case is mostly for if original corefile was empty and rules were also empty.
	// However, the logic above ensures markers are always added.
	return ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress").
		Complete(r)
}
