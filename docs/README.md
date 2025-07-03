![release-please](https://github.com/pelotech/kic/actions/workflows/release-please.yaml/badge.svg)

# KIC
K8s controller to update dns (currently coredns) with ingress configuration to help remove hairpin

# General
This is an early stage project with limited scope

## Assumptions on the k8s cluster
1. tls termination is in cluster - invalid cert issues others
2. uses coredns - future might include additional cluster dns providers

## Running the Application

This application is a Kubernetes controller. It is typically deployed to a Kubernetes cluster using a Docker image.
The `Makefile` provides targets for building and deploying the controller:

```bash
# Build the Docker image
make docker-build

# Deploy the controller to the cluster
make deploy
```

### Command-line Parameters

The controller accepts the following command-line parameters:

| Parameter                      | Description                                                                                          | Default Value                          |
| ------------------------------ | ---------------------------------------------------------------------------------------------------- | -------------------------------------- |
| `metrics-bind-address`         | The address the metrics endpoint binds to. Use `:8443` for HTTPS or `:8080` for HTTP. Set to `0` to disable. | `0`                                    |
| `health-probe-bind-address`    | The address the health probe endpoint binds to.                                                        | `:8081`                                |
| `leader-elect`                 | Enable leader election for controller manager. Ensures only one active controller manager.             | `false`                                |
| `metrics-secure`               | If `true`, the metrics endpoint is served securely via HTTPS. Set to `false` for HTTP.                 | `true`                                 |
| `webhook-cert-path`            | Directory containing the webhook certificate.                                                          | `""`                                   |
| `webhook-cert-name`            | Name of the webhook certificate file.                                                                  | `tls.crt`                              |
| `webhook-cert-key`             | Name of the webhook key file.                                                                          | `tls.key`                              |
| `metrics-cert-path`            | Directory containing the metrics server certificate.                                                   | `""`                                   |
| `metrics-cert-name`            | Name of the metrics server certificate file.                                                           | `tls.crt`                              |
| `metrics-cert-key`             | Name of the metrics server key file.                                                                   | `tls.key`                              |
| `enable-http2`                 | If `true`, HTTP/2 will be enabled for the metrics and webhook servers.                                 | `false`                                |
| `watched-namespaces`           | Comma-separated list of namespaces to watch for Ingresses. If empty, all namespaces are watched.       | `""`                                   |
| `ingress-annotation`           | Annotation to look for on Ingresses. If not set, all Ingresses are considered.                         | `""`                                   |
| `ingress-controller-service`   | Fully qualified domain name of the ingress controller service.                                       | `controller.nginx.svc.cluster.local` |
