![release-please](https://github.com/pelotech/kic/actions/workflows/release-please.yaml/badge.svg)

# KIC
K8s controller to update dns (currently coredns) with ingress configuration to help remove hairpin

# General
This is an early stage project with limited scope

## Assumptions on the k8s cluster
1. tls termination is in cluster - invalid cert issues others
2. uses coredns - future might include additional cluster dns providers

