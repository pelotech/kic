package controller

import (
	"strings"
	"testing"
)

func TestInjectRewriteRules(t *testing.T) {
	tests := []struct {
		name             string
		corefile         string
		newRules         string
		expectedCorefile string
	}{
		{
			name:     "empty corefile, no new rules",
			corefile: "",
			newRules: "",
			expectedCorefile: managedRulesBeginMarker + "\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name:     "empty corefile, with new rules",
			corefile: "",
			newRules: "rewrite name host1 service1\nrewrite name host2 service2",
			expectedCorefile: managedRulesBeginMarker + "\n" +
				"rewrite name host1 service1\n" +
				"rewrite name host2 service2\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name: "corefile without markers, no new rules",
			corefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"        fallthrough in-addr.arpa ip6.arpa\n" +
				"    }\n" +
				"    forward . /etc/resolv.conf\n" +
				"    cache 30\n" +
				"    loop\n" +
				"    reload\n" +
				"    loadbalance\n" +
				"}\n",
			newRules: "",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"        fallthrough in-addr.arpa ip6.arpa\n" +
				"    }\n" +
				managedRulesBeginMarker + "\n" +
				managedRulesEndMarker + "\n" +
				"    forward . /etc/resolv.conf\n" +
				"    cache 30\n" +
				"    loop\n" +
				"    reload\n" +
				"    loadbalance\n" +
				"}\n",
		},
		{
			name: "corefile without markers, with new rules",
			corefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"        fallthrough in-addr.arpa ip6.arpa\n" +
				"    }\n" +
				"    forward . /etc/resolv.conf\n" +
				"    cache 30\n" +
				"    loop\n" +
				"    reload\n" +
				"    loadbalance\n" +
				"}\n",
			newRules: "rewrite name host1 service1",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"        fallthrough in-addr.arpa ip6.arpa\n" +
				"    }\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name host1 service1\n" +
				managedRulesEndMarker + "\n" +
				"    forward . /etc/resolv.conf\n" +
				"    cache 30\n" +
				"    loop\n" +
				"    reload\n" +
				"    loadbalance\n" +
				"}\n",
		},
		{
			name: "corefile with existing markers, no new rules (clear existing)",
			corefile: ".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				"    rewrite name oldhost oldservice\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n",
			newRules: "",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n",
		},
		{
			name: "corefile with existing markers, with new rules (replace existing)",
			corefile: ".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				"    rewrite name oldhost oldservice\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n",
			newRules: "rewrite name newhost newservice",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name newhost newservice\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n",
		},
		{
			name: "corefile with existing markers and other rewrites, replace rules",
			corefile: "rewrite name external external.service\n" +
				".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				"    rewrite name oldhost oldservice\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n" +
				"rewrite name another.external another.service\n",
			newRules: "rewrite name newhost newservice",
			expectedCorefile: "rewrite name external external.service\n" +
				".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name newhost newservice\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n" +
				"rewrite name another.external another.service\n",
		},
		{
			name: "corefile with markers at the end, add new rules",
			corefile: ".:53 {\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa\n" +
				"}\n" +
				managedRulesBeginMarker + "\n" +
				managedRulesEndMarker + "\n",
			newRules: "rewrite name host.end end.svc",
			expectedCorefile: ".:53 {\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa\n" +
				"}\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name host.end end.svc\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name: "corefile with markers and rules, new rules are multi-line",
			corefile: managedRulesBeginMarker + "\n" +
				"rewrite name old1 s1\n" +
				managedRulesEndMarker + "\n",
			newRules: "rewrite name new1 s1\nrewrite name new2 s2",
			expectedCorefile: managedRulesBeginMarker + "\n" +
				"rewrite name new1 s1\n" +
				"rewrite name new2 s2\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name:     "Corefile with only markers, no newlines, add rules",
			corefile: managedRulesBeginMarker + managedRulesEndMarker,
			newRules: "rewrite name h c",
			// This case normalizes the markers to have newlines
			expectedCorefile: managedRulesBeginMarker + "\n" +
				"rewrite name h c\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name: "Corefile with one rule, then markers, then one rule",
			corefile: "rewrite name pre pre.svc\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name old old.svc\n" +
				managedRulesEndMarker + "\n" +
				"rewrite name post post.svc\n",
			newRules: "rewrite name new new.svc",
			expectedCorefile: "rewrite name pre pre.svc\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name new new.svc\n" +
				managedRulesEndMarker + "\n" +
				"rewrite name post post.svc\n",
		},
		{
			name: "Injection when kubernetes plugin has config block",
			corefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"        fallthrough in-addr.arpa ip6.arpa\n" +
				"        ttl 30\n" +
				"    }\n" +
				"    forward . /etc/resolv.conf\n" +
				"}\n",
			newRules: "rewrite name k8s.block k8s.svc",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"        fallthrough in-addr.arpa ip6.arpa\n" +
				"        ttl 30\n" +
				"    }\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name k8s.block k8s.svc\n" +
				managedRulesEndMarker + "\n" +
				"    forward . /etc/resolv.conf\n" +
				"}\n",
		},
		{
			name: "Corefile with no kubernetes plugin (fallback to append)",
			corefile: ".:53 {\n" +
				"    forward . /etc/resolv.conf\n" +
				"}\n",
			newRules: "rewrite name no.k8s no.k8s.svc",
			expectedCorefile: ".:53 {\n" +
				"    forward . /etc/resolv.conf\n" +
				"}\n" + // Trailing newline from original processing
				managedRulesBeginMarker + "\n" +
				"rewrite name no.k8s no.k8s.svc\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name:     "Corefile completely empty, inject new rules with markers",
			corefile: "",
			newRules: "rewrite name example example.com",
			expectedCorefile: managedRulesBeginMarker + "\n" +
				"rewrite name example example.com\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name:     "Corefile with only a comment, inject new rules with markers",
			corefile: "# This is a comment\n",
			newRules: "rewrite name commented commented.svc",
			// The current logic appends if no kubernetes directive is found
			expectedCorefile: "# This is a comment\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name commented commented.svc\n" +
				managedRulesEndMarker + "\n",
		},
		{
			name: "Corefile with markers but no newline between them",
			corefile: ".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + managedRulesEndMarker + "\n" +
				"    health\n" +
				"}\n",
			newRules: "rewrite name tight tight.svc",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				managedRulesBeginMarker + "\n" +
				"rewrite name tight tight.svc\n" +
				managedRulesEndMarker + "\n" +
				"    health\n" +
				"}\n",
		},
		{
			name: "corefile without metadata, with expression rules",
			corefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n",
			newRules: "expression \"!(label('kubernetes/client-namespace') in ['kube-system'])\" {\n" +
				"    rewrite name host1 service1\n" +
				"}",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"    metadata\n" +
				managedRulesBeginMarker + "\n" +
				"expression \"!(label('kubernetes/client-namespace') in ['kube-system'])\" {\n" +
				"    rewrite name host1 service1\n" +
				"}\n" +
				managedRulesEndMarker + "\n" +
				"}\n",
		},
		{
			name: "corefile with metadata, with expression rules",
			corefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    metadata\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				"}\n",
			newRules: "expression \"!(label('kubernetes/client-namespace') in ['kube-system'])\" {\n" +
				"    rewrite name host1 service1\n" +
				"}",
			expectedCorefile: ".:53 {\n" +
				"    errors\n" +
				"    health\n" +
				"    metadata\n" +
				"    kubernetes cluster.local in-addr.arpa ip6.arpa {\n" +
				"        pods insecure\n" +
				"    }\n" +
				managedRulesBeginMarker + "\n" +
				"expression \"!(label('kubernetes/client-namespace') in ['kube-system'])\" {\n" +
				"    rewrite name host1 service1\n" +
				"}\n" +
				managedRulesEndMarker + "\n" +
				"}\n",
		},
	}

	r := &IngressReconciler{} // Dummy reconciler for this test

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize input corefile to have a trailing newline if not empty, like real Corefiles often do
			corefileInput := tt.corefile
			if corefileInput != "" && !strings.HasSuffix(corefileInput, "\n") {
				corefileInput += "\n"
			}
			// Also trim trailing newlines from expected for consistent comparison, then add one back if not empty
			expected := strings.TrimSpace(tt.expectedCorefile)
			if expected != "" {
				expected += "\n"
			}
			if tt.expectedCorefile == "" { // if expected is truly empty string
				expected = ""
			}

			actual := r.injectRewriteRules(corefileInput, tt.newRules)
			actual = strings.TrimSpace(actual) // Trim for comparison
			if actual != "" {
				actual += "\n"
			}

			if strings.TrimSpace(actual) != strings.TrimSpace(expected) {
				t.Errorf("injectRewriteRules() for '%s':\nExpected:\n```\n%s```\nActual:\n```\n%s```", tt.name, expected, actual)
			}
		})
	}
}
