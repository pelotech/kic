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
	coreDNSConfigMapNamespace = "kube-system"
	corefileKey               = "Corefile"
	rewriteRuleFormat         = "rewrite name %s %s\n"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Log                      logr.Logger
	Scheme                   *runtime.Scheme
	IngressAnnotation        string
	IngressControllerService string
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
				newRewriteRules.WriteString(fmt.Sprintf(rewriteRuleFormat, rule.Host, r.IngressControllerService))
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
	// A simple strategy to inject rules.
	// This finds the main server block and injects the rules.
	// A more robust implementation would parse the Corefile more intelligently.
	lines := strings.Split(corefile, "\n")
	var newCorefile strings.Builder
	inServerBlock := false
	rewritesInjected := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, ".:53") {
			inServerBlock = true
		}

		if inServerBlock && !rewritesInjected {
			if strings.HasPrefix(trimmedLine, "kubernetes") {
				newCorefile.WriteString(newRules)
				newCorefile.WriteString("\n")
				rewritesInjected = true
			}
		}
		if !strings.HasPrefix(trimmedLine, "rewrite name") {
			newCorefile.WriteString(line)
			newCorefile.WriteString("\n")
		}
	}

	// If the server block wasn't found, append at the end (less ideal).
	if !rewritesInjected {
		return corefile + "\n" + newRules
	}

	return newCorefile.String()
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress").
		Complete(r)
}
