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
	"regexp"
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
	coreDNSConfigMapNamespace = "kube-system"
	corefileKey               = "Corefile"
	rewriteRuleFormat         = "rewrite name %s %s\n"
	managedRulesBeginMarker   = "# BEGIN IngressReconciler managed rules"
	managedRulesEndMarker     = "# END IngressReconciler managed rules"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Log                          logr.Logger
	Scheme                       *runtime.Scheme
	IngressAnnotation            string
	IngressControllerServiceName string
	CoreDNSExcludedNamespaces    []string
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

	rulesString := newRewriteRules.String()
	// If there are excluded namespaces, wrap the rules in an expression
	if len(r.CoreDNSExcludedNamespaces) > 0 && rulesString != "" {
		// Format each namespace as a quoted string
		quotedNamespaces := make([]string, len(r.CoreDNSExcludedNamespaces))
		for i, ns := range r.CoreDNSExcludedNamespaces {
			quotedNamespaces[i] = fmt.Sprintf("'%s'", ns)
		}
		// Create the CEL expression
		expression := fmt.Sprintf("!(label('kubernetes/client-namespace') in [%s])", strings.Join(quotedNamespaces, ", "))
		// Wrap the rules in the expression block
		rulesString = fmt.Sprintf("expression \"%s\" {\n%s}\n", expression, strings.TrimSpace(rulesString))
	}

	updatedCorefile := r.injectRewriteRules(originalCorefile, rulesString)

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
// and returns the modified Corefile content. It uses a regex-based approach to manage a
// demarcated block of rules.
func (r *IngressReconciler) injectRewriteRules(corefileContent string, newRules string) string {
	// 1. Prepare the new managed block that should be in the Corefile.
	var newManagedBlock strings.Builder
	newManagedBlock.WriteString(managedRulesBeginMarker + "\n")
	if newRules != "" {
		newManagedBlock.WriteString(strings.TrimSpace(newRules) + "\n")
	}
	newManagedBlock.WriteString(managedRulesEndMarker)
	blockToAdd := newManagedBlock.String()

	// 2. Define a regex to find an existing managed block.
	// The `(?s)` flag allows `.` to match newlines.
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(managedRulesBeginMarker) + `.*` + regexp.QuoteMeta(managedRulesEndMarker) + `\n?`)

	var updatedCorefile string
	if re.MatchString(corefileContent) {
		// Case 1: An existing block is found. Replace it with the new one.
		updatedCorefile = re.ReplaceAllString(corefileContent, blockToAdd+"\n")
	} else {
		// Case 2: No existing block found. We need to inject it.
		// We'll try to inject it right after the 'kubernetes' plugin for neatness.
		lines := strings.Split(corefileContent, "\n")
		insertionPoint := -1

		kubernetesLine := -1
		for i, line := range lines {
			if strings.Contains(line, "kubernetes") {
				kubernetesLine = i
				break
			}
		}

		if kubernetesLine != -1 {
			// We found the line with the kubernetes plugin.
			// Now, find the end of its configuration block.
			if !strings.Contains(lines[kubernetesLine], "{") {
				// It's a single-line declaration. Insert on the next line.
				insertionPoint = kubernetesLine + 1
			} else {
				// It's a multi-line block. We need to find the matching closing brace.
				openBraces := 0
				for i := kubernetesLine; i < len(lines); i++ {
					openBraces += strings.Count(lines[i], "{")
					openBraces -= strings.Count(lines[i], "}")
					if openBraces == 0 {
						// We found the closing brace on this line. Insert after it.
						insertionPoint = i + 1
						break
					}
				}
				if insertionPoint == -1 {
					// This case means a malformed Corefile with unclosed braces.
					// Fallback to appending at the end.
					kubernetesLine = -1 // This will trigger the fallback logic.
				}
			}
		}

		var newCorefileBuilder strings.Builder
		if insertionPoint != -1 {
			// Inject after the 'kubernetes' plugin block.
			newCorefileBuilder.WriteString(strings.Join(lines[:insertionPoint], "\n"))
			if newCorefileBuilder.Len() > 0 {
				newCorefileBuilder.WriteString("\n")
			}
			newCorefileBuilder.WriteString(blockToAdd)
			if insertionPoint < len(lines) {
				newCorefileBuilder.WriteString("\n")
				newCorefileBuilder.WriteString(strings.Join(lines[insertionPoint:], "\n"))
			}
		} else {
			// Fallback: Append to the end of the file.
			trimmedContent := strings.TrimSpace(corefileContent)
			if trimmedContent != "" {
				newCorefileBuilder.WriteString(trimmedContent)
				newCorefileBuilder.WriteString("\n")
			}
			newCorefileBuilder.WriteString(blockToAdd)
		}
		updatedCorefile = newCorefileBuilder.String()
	}

	// 3. Ensure the 'metadata' plugin is present if needed.
	needsMetadata := strings.Contains(newRules, "expression")
	// Check the updated content for the metadata plugin.
	hasMetadata := strings.Contains(updatedCorefile, "metadata")

	finalCorefile := updatedCorefile
	if needsMetadata && !hasMetadata {
		// If metadata is needed but not present, inject it before our managed block.
		finalCorefile = strings.Replace(updatedCorefile, managedRulesBeginMarker, "    metadata\n"+managedRulesBeginMarker, 1)
	}

	// 4. Normalize and return the final content.
	finalOutput := strings.TrimSpace(finalCorefile)
	if finalOutput != "" {
		return finalOutput + "\n"
	}
	return ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress").
		Complete(r)
}
