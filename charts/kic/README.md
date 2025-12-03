# kic

### version: 0.2.2<!-- x-release-please-version -->

### appVersion: v0.2.2 <!-- x-release-please-version -->

A Helm chart for Kubernetes Image Cacher (kic)

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| autoscaling.enabled | bool | `false` |  |
| autoscaling.maxReplicas | int | `100` |  |
| autoscaling.minReplicas | int | `1` |  |
| autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| controllerManager | object | `{"corednsExcludedNamespaces":"","enableHttp2":false,"health":{"bindAddress":":8081"},"ingressAnnotation":"","ingressControllerService":"ingress-nginx-controller.ingress-nginx.svc.cluster.local","leaderElect":false,"metrics":{"bindAddress":":8080","secure":false},"watchedNamespaces":""}` | Controller manager specific settings |
| controllerManager.corednsExcludedNamespaces | string | `""` | Comma-separated list of namespaces to ignore custom rewrite rules. |
| controllerManager.enableHttp2 | bool | `false` | Enable HTTP2 for metrics and webhook servers. |
| controllerManager.health | object | `{"bindAddress":":8081"}` | Health probe settings |
| controllerManager.health.bindAddress | string | `":8081"` | Address to bind health probe endpoint to. |
| controllerManager.ingressAnnotation | string | `""` | Annotation to look for on Ingresses. Empty means all Ingresses. |
| controllerManager.ingressControllerService | string | `"ingress-nginx-controller.ingress-nginx.svc.cluster.local"` | Fully qualified domain name of the ingress controller service. |
| controllerManager.metrics | object | `{"bindAddress":":8080","secure":false}` | Metrics settings |
| controllerManager.metrics.bindAddress | string | `":8080"` | Address to bind metrics endpoint to. Set to "0" to disable. |
| controllerManager.metrics.secure | bool | `false` | Whether to serve metrics securely (HTTPS). Requires certs if true and bindAddress is not "0". |
| controllerManager.watchedNamespaces | string | `""` | Comma-separated list of namespaces to watch. Empty means all namespaces. |
| env | list | `[]` |  |
| extraArgs | list | `[]` |  |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/pelotech/kic"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| ingress.annotations | object | `{}` |  |
| ingress.className | string | `""` |  |
| ingress.enabled | bool | `false` |  |
| ingress.hosts[0].host | string | `"chart-example.local"` |  |
| ingress.hosts[0].paths[0].path | string | `"/"` |  |
| ingress.hosts[0].paths[0].pathType | string | `"ImplementationSpecific"` |  |
| ingress.tls | list | `[]` |  |
| livenessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz"},"initialDelaySeconds":15,"periodSeconds":20,"timeoutSeconds":5}` | Liveness probe configuration |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext | object | `{}` |  |
| readinessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/readyz"},"initialDelaySeconds":5,"periodSeconds":10,"timeoutSeconds":5}` | Readiness probe configuration |
| replicaCount | int | `1` |  |
| resources | object | `{}` |  |
| securityContext | object | `{}` |  |
| service.annotations | object | `{}` |  |
| service.enabled | string | `"auto"` |  |
| service.port | int | `8080` |  |
| service.targetPort | string | `"metrics"` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| tolerations | list | `[]` |  |

