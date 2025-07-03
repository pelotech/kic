![release-please](https://github.com/pelotech/kic/actions/workflows/release-please.yaml/badge.svg)

# KIC
K8s controller to update dns (currently coredns) with ingress configuration to help remove hairpin

# General
This is an early stage project with limited scope

## Assumptions on the k8s cluster
1. tls termination is in cluster - invalid cert issues others
2. uses coredns - future might include additional cluster dns providers

---

## Helm Chart

This project includes a Helm chart located in the `charts/kic` directory to deploy KIC to your Kubernetes cluster.

### Prerequisites

*   Kubernetes 1.19+
*   Helm 3.2.0+

### Installing the Chart

To install the chart with the release name `my-kic`:

```bash
helm repo add pelotech https://pelotech.github.io/helm-charts # (Assuming you publish it here, otherwise use local path)
helm install my-kic pelotech/kic --namespace your-namespace --create-namespace
```

Or, to install from a local path:

```bash
helm install my-kic ./charts/kic --namespace your-namespace --create-namespace
```

### Uninstalling the Chart

To uninstall/delete the `my-kic` deployment:

```bash
helm uninstall my-kic --namespace your-namespace
```

### Configuration

The following table lists the configurable parameters of the KIC chart and their default values.

| Parameter                                      | Description                                                                                                | Default                                       |
| ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `replicaCount`                                 | Number of KIC replicas.                                                                                    | `1`                                           |
| `image.repository`                             | Image repository for KIC.                                                                                  | `ghcr.io/pelotech/kic`                        |
| `image.pullPolicy`                             | Image pull policy.                                                                                         | `IfNotPresent`                                |
| `image.tag`                                    | Image tag (overrides chart's `appVersion` if set).                                                         | Chart's `appVersion` (`latest`)               |
| `imagePullSecrets`                             | Array of image pull secrets.                                                                               | `[]`                                          |
| `nameOverride`                                 | String to override the default chart name.                                                                 | `""`                                          |
| `fullnameOverride`                             | String to override the fully qualified application name.                                                   | `""`                                          |
| `extraArgs`                                    | Array of extra command-line arguments to pass to the KIC container.                                        | `[]`                                          |
| `env`                                          | Array of environment variables to set in the KIC container.                                                | `[]`                                          |
| `controllerManager.leaderElect`                | Enable leader election for the controller manager.                                                         | `false`                                       |
| `controllerManager.metrics.bindAddress`        | Address for the metrics endpoint (e.g., `:8080`, `:8443`). Set to `0` to disable.                           | `:8080`                                       |
| `controllerManager.metrics.secure`             | If `true`, serve metrics via HTTPS (requires certs if `bindAddress` is not `0`).                           | `false`                                       |
| `controllerManager.health.bindAddress`         | Address for the health probe endpoint (e.g., `:8081`).                                                     | `:8081`                                       |
| `controllerManager.watchedNamespaces`          | Comma-separated list of namespaces to watch. Empty means all namespaces.                                   | `""`                                          |
| `controllerManager.ingressAnnotation`          | Annotation to look for on Ingresses. Empty means all Ingresses.                                            | `""`                                          |
| `controllerManager.ingressControllerService`   | Fully qualified domain name of the ingress controller service.                                             | `controller.nginx.svc.cluster.local`          |
| `controllerManager.enableHttp2`                | Enable HTTP/2 for metrics and webhook servers.                                                             | `false`                                       |
| `livenessProbe.httpGet.path`                   | Path for the liveness probe.                                                                               | `/healthz`                                    |
| `livenessProbe.initialDelaySeconds`            | Initial delay for the liveness probe.                                                                      | `15`                                          |
| `livenessProbe.periodSeconds`                  | Period for the liveness probe.                                                                             | `20`                                          |
| `livenessProbe.timeoutSeconds`                 | Timeout for the liveness probe.                                                                            | `5`                                           |
| `livenessProbe.failureThreshold`               | Failure threshold for the liveness probe.                                                                  | `3`                                           |
| `readinessProbe.httpGet.path`                  | Path for the readiness probe.                                                                              | `/readyz`                                     |
| `readinessProbe.initialDelaySeconds`           | Initial delay for the readiness probe.                                                                     | `5`                                           |
| `readinessProbe.periodSeconds`                 | Period for the readiness probe.                                                                            | `10`                                          |
| `readinessProbe.timeoutSeconds`                | Timeout for the readiness probe.                                                                           | `5`                                           |
| `readinessProbe.failureThreshold`              | Failure threshold for the readiness probe.                                                                 | `3`                                           |
| `serviceAccount.create`                        | Specifies whether a service account should be created.                                                     | `true`                                        |
| `serviceAccount.annotations`                   | Annotations to add to the service account.                                                                 | `{}`                                          |
| `serviceAccount.name`                          | The name of the service account to use. If not set and `create` is true, a name is generated.              | `""`                                          |
| `podAnnotations`                               | Annotations to add to the KIC pod.                                                                         | `{}`                                          |
| `podLabels`                                    | Labels to add to the KIC pod.                                                                              | `{}`                                          |
| `podSecurityContext`                           | Security context for the KIC pod.                                                                          | `{}`                                          |
| `securityContext`                              | Security context for the KIC container.                                                                    | `{}`                                          |
| `service.enabled`                              | Enable the metrics service. Can be `true`, `false`, or `"auto"` (enables if metrics are configured).       | `"auto"`                                      |
| `service.type`                                 | Type of service for metrics (e.g., `ClusterIP`, `NodePort`).                                               | `ClusterIP`                                   |
| `service.port`                                 | Port for the metrics service.                                                                              | `8080`                                        |
| `service.targetPort`                           | Target port on the pod for the metrics service.                                                            | `metrics`                                     |
| `service.annotations`                          | Annotations for the metrics service.                                                                       | `{}`                                          |
| `resources`                                    | CPU/memory resource requests and limits for the KIC container.                                             | `{}` (no defaults)                            |
| `nodeSelector`                                 | Node selector for pod assignment.                                                                          | `{}`                                          |
| `tolerations`                                  | Tolerations for pod assignment.                                                                            | `[]`                                          |
| `affinity`                                     | Affinity for pod assignment.                                                                               | `{}`                                          |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` or `helm upgrade`. For example:

```bash
helm install my-kic ./charts/kic \
  --set controllerManager.leaderElect=true \
  --set controllerManager.metrics.bindAddress=:8443 \
  --set controllerManager.metrics.secure=true
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example:

```bash
helm install my-kic ./charts/kic -f my-values.yaml
```

> **Tip**: You can use `helm show values ./charts/kic` to see all configurable values.
