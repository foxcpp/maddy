# maddy Helm chart for Kubernetes

![Version: 0.2.5](https://img.shields.io/badge/Version-0.2.5-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.4.1](https://img.shields.io/badge/AppVersion-0.4.1-informational?style=flat-square)

This is just initial effort to run maddy within Kubernetes cluster. We have used Deployment resource which has some downsides
but at least this chart will allow you to install maddy relatively easily on your Kubernetes cluster. We have considered
StatefulSet and DaemonSet but such solutions would require much more configuration and in casae of DaemonSet also a TCP
load balancer in front of the nodes.

## Requirement

In order to run maddy properly, you need to have TLS secret under name maddy present in the cluster. If you have commercial
certificate, you can create it by the following command:

```sh
kubectl create secret tls maddy --cert=fullchain.pem --key=privkey.pem
```

If you use cert-manager, just create the secret under name maddy.

## Replication

Default for this chart is 1 replica of maddy. If you try to increase this, you will probably get an error because of
the busy ports 25, 143, 587, etc. We do not support this feature at the moment, so please use just 1 replica. Like said
at the beginning of this document, multiple replicas would probably require to switch do DaemonSet which would further require
to have TCP load balancer and shared storage between all replicas. This is not supported by this chart, sorry.
This chart is used on one node cluster and then installation is straight forward, like described bellow, but if you have
multiple node cluster, please use taints and tolerations to select the desired node. This chart supports tolerations to
be set.

## Configuration

| Key                        | Type   | Default           | Description |
| -------------------------- | ------ | ----------------- | ----------- |
| affinity                   | object | `{}`              |             |
| fullnameOverride           | string | `""`              |             |
| image.pullPolicy           | string | `"IfNotPresent"`  |             |
| image.repository           | string | `"foxcpp/maddy"`  |             |
| image.tag                  | string | `""`              |             |
| imagePullSecrets           | list   | `[]`              |             |
| nameOverride               | string | `""`              |             |
| nodeSelector               | object | `{}`              |             |
| persistence.accessMode     | string | `"ReadWriteOnce"` |             |
| persistence.annotations    | object | `{}`              |             |
| persistence.enabled        | bool   | `false`           |             |
| persistence.path           | string | `"/data"`         |             |
| persistence.size           | string | `"128Mi"`         |             |
| podAnnotations             | object | `{}`              |             |
| podSecurityContext         | object | `{}`              |             |
| replicaCount               | int    | `1`               |             |
| resources                  | object | `{}`              |             |
| securityContext            | object | `{}`              |             |
| service.type               | string | `"NodePort"`      |             |
| serviceAccount.annotations | object | `{}`              |             |
| serviceAccount.create      | bool   | `true`            |             |
| serviceAccount.name        | string | `""`              |             |
| tolerations                | list   | `[]`              |             |

## Installing the chart

```sh
helm upgrade --install maddy ./chart --set service.externapIPs[0]=1.2.3.4
```

1.2.3.4 is your public IP of the node.

## maddy configuration

Feel free to tweak files/maddy.conf and files/aliases according to your needs.
