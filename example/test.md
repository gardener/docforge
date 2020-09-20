---
Title: Test
---

# Gardenlet

Right from the beginning of the Gardener Project we started implementing the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/):
We have a custom controller-manager that acts on our own custom resources.
Now, when you start thinking about the [Gardener architecture](https://github.com/gardener/documentation/wiki/Architecture), you will recognize some interesting similarity with respect to the Kubernetes architecture:
Shoot clusters can be compared with pods, and seed clusters can be seen as worker nodes.
Guided by this observation we introduced the **gardener-scheduler** ([#356](https://github.com/gardener/gardener/issues/356)).

Yet, there was still a significant difference between the Kubernetes and the Gardener architectures:
Kubernetes runs a primary "agent" on every node, the kubelet, which is mainly responsible for managing pods and containers on its particular node.
Gardener used its single controller-manager which was responsible for all shoot clusters on all seed clusters, and it was performing its reconciliation loops centrally from the garden cluster.

![](gardenlet-architecture-similarities.png)

The gardener-controller-manager still kept its control loops for other resources of the Gardener API, however, it does no longer talk to seed/shoot clusters.

## TLS Bootstrapping

As explained in the above motivation, the Gardenlet can be compared with the kubelet.
After you depoyed it yourself into your seed clusters, it initializes a bootstrapping process that is very similar to the [Kubelet's TLS bootstrapping](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/):

* Gardenlet starts up with a bootstrap kubeconfig having a bootstrap token that allows to create `CertificateSigningRequest` resources
* After the CSR is signed it downloads the created client certificate, creates a new kubeconfig with it, and stores it inside a `Secret` in the seed cluster
* It deletes the bootstrap kubeconfig secret and starts up with its new kubeconfig

Basically, you can follow the Kubernetes documentation regarding the process.
The gardener-controller-manager runs a control loop that automatically signs CSRs created by Gardenlets.

:warning: Certificate rotation is not yet implemented but will follow very soon.

Optionally, if you don't want to run this bootstrap process, then you can create a Kubeconfig pointing to the Garden cluster for the Gardenlet yourself, and simply provide it to it.

## Seed Config vs. Seed Selector

If you want the Gardenlet in the standard way then please provide a `seedConfig` that contains information about the seed cluster itself, see [the example Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml#L69-L102).

## Shooted Seeds

If the Gardenlet manages a shoot cluster that has been marked to be used as seed then it will automatically deploy itself into the cluster, unless you prevent this by using the `no-gardenlet` configuration in the `shoot.gardener.cloud/use-as-seed` annotation (in this case, you have to deploy the Gardenlet on your own into the seed cluster).

*Example: Annotate the shoot with `shoot.gardener.cloud/use-as-seed="true,no-gardenlet,invisible"` to mark it as invisible (meaning, that the gardener-scheduler won't consider it) and to express your desire to deploy the gardenlet into the cluster on your own.*

For automatic deployments, the Gardenlet will use the same version and the same configuration for the clone it deploys.

https://github.com/gardenber/gardener

<p>

  [asds](https://github.com/gardener/gardener)

  <a href="https://github.com">alabala</a> <img src="../images/logo.png">

</p>

|Alertname|Severity|Type|Description|
|---|---|---|---|
|ApiServerUnreachableViaKubernetesService|critical|shoot|`The Api server has been unreachable for 3 minutes via the kubernetes service in the shoot.`|
|CoreDNSDown|critical|shoot|`CoreDNS could not be found. Cluster DNS resolution will not work.`|