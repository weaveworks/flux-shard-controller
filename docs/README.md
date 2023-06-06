# Getting Started with the Flux Shard Controller

Flux introduced a [sharding mechanism](https://fluxcd.io/flux/cheatsheets/sharding/) which allows users to configure controllers to apply resources with specific labels.

Simply, this allows you to label Flux resources e.g. [GitRepository](https://fluxcd.io/flux/components/source/gitrepositories/) and have it be processed by a specific controller.

The Flux sharding documentation describes bootstrapping Flux for specific shards, the Weave GitOps Sharding Controller can automate the lifecycle of the shard-controllers, which means you can add and remove new shards without having to update the Flux components.

## Bootstrapping for Automatic Shard deployments

First you'll need to create a Git repository and clone it locally, then
create the file structure required by bootstrap with:

```sh
mkdir -p clusters/my-cluster/flux-system
touch clusters/my-cluster/flux-system/gotk-components.yaml \
    clusters/my-cluster/flux-system/gotk-sync.yaml \
    clusters/my-cluster/flux-system/kustomization.yaml
```

Add `clusters/my-cluster/flux-system/kustomization.yaml` file to configure the main Flux controllers to ignore sharding:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- gotk-components.yaml
- gotk-sync.yaml
patches:
  - target:
      kind: Deployment
      name: "(source-controller|kustomize-controller|helm-controller)"
      annotationSelector: "sharding.fluxcd.io/role notin (shard)"
    patch: |
      - op: add
        path: /spec/template/spec/containers/0/args/0
        value: --watch-label-selector=!sharding.fluxcd.io/key
```

Note how this configuration excludes the sharding keys from the main controllers
watch with `--watch-label-selector=!sharding.fluxcd.io/key`. This ensures that
the main controllers will not reconcile any Flux resources labels with sharding keys.

### Install shards

Push the changes to main branch:

```sh
git add -A && git commit -m "init flux" && git push
```

And run the bootstrap for `clusters/my-cluster`:

```sh
flux bootstrap git \
  --url=ssh://git@<host>/<org>/<repository> \
  --branch=main \
  --path=clusters/my-cluster
```

Verify that the main controllers are running:

```console
$ kubectl get deployments -n flux-system
NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
helm-controller           1/1     1            1           3m59s
kustomize-controller      1/1     1            1           3m59s
notification-controller   1/1     1            1           3m59s
source-controller         1/1     1            1           3m59s
```

We can also verify that the `helm-controller`, `kustomize-controller` and `source-controller` are configured to ignore shards:

```console
$ kubectl get deploy -n flux-system -o=jsonpath='{range .items[*]}{.metadata.name}{.spec.template.spec.containers[0].args}{"\n"}{end}'
helm-controller["--watch-label-selector=!sharding.fluxcd.io/key","--events-addr=http://notification-controller.flux-system.svc.cluster.local./","--watch-all-namespaces=true","--log-level=info","--log-encoding=json","--enable-leader-election"]
kustomize-controller["--watch-label-selector=!sharding.fluxcd.io/key","--events-addr=http://notification-controller.flux-system.svc.cluster.local./","--watch-all-namespaces=true","--log-level=info","--log-encoding=json","--enable-leader-election"]
source-controller["--watch-label-selector=!sharding.fluxcd.io/key","--events-addr=http://notification-controller.flux-system.svc.cluster.local./","--watch-all-namespaces=true","--log-level=info","--log-encoding=json","--enable-leader-election","--storage-path=/data","--storage-adv-addr=source-controller.$(RUNTIME_NAMESPACE).svc.cluster.local."]
```

## Install the Shard Controller

**TODO**

## Create a non-sharded GitRepository

```sh
mkdir -p clusters/my-cluster/demo
```

Create a file in `clusters/my-cluster/demo/gitrepository.yaml`

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: podinfo
  namespace: default
spec:
  interval: 5m0s
  url: https://github.com/stefanprodan/podinfo
  ref:
    branch: master
```

Now push this change for Flux to sync into the cluster:

```sh
git add -A && git commit -m "Add simple unsharded repository." && git push
```

You can wait for the changes to be picked up automatically, or trigger a manual reconciliation:

```console
 $ flux reconcile source git flux-system
► annotating GitRepository flux-system in flux-system namespace
✔ GitRepository annotated
◎ waiting for GitRepository reconciliation
✔ fetched revision main@sha1:77d5316fa6e7c48f0f2207d9cfd7d6398d1d02e0
```

To confirm that the repository has been processed by the main source-controller:

```console
$ kubectl logs deploy/source-controller -n flux-system
{"level":"info","ts":"2023-06-29T09:52:58.228Z","msg":"stored artifact for commit 'Add simple unsharded repository.'","controller":"gitrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"GitRepository","GitRepository":{"name":"flux-system","namespace":"flux-system"},"namespace":"flux-system","name":"flux-system","reconcileID":"b1836eb8-f2f5-434d-8009-edefb35a4ae9"}
```

### Configure Shards

First let's create a new ShardSet in `clusters/my-cluster/demo/source-controller-shardset.yaml`:

```yaml
apiVersion: templates.weave.works/v1alpha1
kind: FluxShardSet
metadata:
  name: source-controller-shardset
  namespace: flux-system
spec:
  sourceDeploymentRef:
    name: source-controller
  shards:
    - name: shard1
```

Now push this change for Flux to sync into the cluster:

```sh
git add -A && git commit -m "Add Shardset." && git push
```

And again, you can wait, or trigger a manual reconciliation.

When the FluxShardSet has synced into the cluster, you can see the state:

```console
$ kubectl get fluxshardsets -n flux-system
NAME                         READY   STATUS
source-controller-shardset   True    1 shard(s) created
```

You should now have a deployment that is processing `shard1` resources.

```console
$ kubectl get deploy -n flux-system
NAME                               READY   UP-TO-DATE   AVAILABLE   AGE
flux-sharding-controller-manager   1/1     1            1           73m
helm-controller                    1/1     1            1           123m
kustomize-controller               1/1     1            1           123m
notification-controller            1/1     1            1           123m
source-controller                  1/1     1            1           123m
source-controller-shard1           1/1     1            1           55s
```

The shard1 controller is configured to process sources with a shard1 label.

```console
$ kubectl get deploy/source-controller-shard1 -n flux-system -o=jsonpath='{.spec.template.spec.containers[0].args}{"\n"}'
["--watch-label-selector=sharding.fluxcd.io/key in (shard1)","--events-addr=http://notification-controller.flux-system.svc.cluster.local./","--watch-all-namespaces=true","--log-level=info","--log-encoding=json","--enable-leader-election","--storage-path=/data","--storage-adv-addr=source-controller.$(RUNTIME_NAMESPACE).svc.cluster.local."]
```

### Create a Sharded GitRepository

Create a new `GitRepository` in `clusters/my-cluster/demo/gitrepository_with_shard1.yaml`:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: podinfo-shard1
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard1
spec:
  interval: 5m0s
  url: https://github.com/stefanprodan/podinfo
  ref:
    branch: v6.x
```

Again push this change for Flux to sync into the cluster:

```sh
git add -A && git commit -m "Add Sharded GitRepository." && git push
```

As before, you can wait, or trigger a manual reconciliation.

```console
$ kubectl get gitrepositories
NAME             URL                                       AGE   READY   STATUS
podinfo          https://github.com/stefanprodan/podinfo   12m   True    stored artifact for revision 'master@sha1:dd3869b1a177432b60ea1e3ba99c10fc9db850fa'
podinfo-shard1   https://github.com/stefanprodan/podinfo   3s    True    stored artifact for revision 'v6.x@sha1:f1f846bf51299c2c12150a0f69609cb99dd94995'
```

Now, we can check that the logs indicate which controller processed this `GitRepository`.

The original source-controller hasn't processed this update:

```console
$ k logs deploy/source-controller -n flux-system | tail -3
Found 2 pods, using pod/source-controller-646c9b88f4-8fwzw
{"level":"info","ts":"2023-06-29T10:21:19.345Z","msg":"stored artifact for commit 'Add Sharded GitRepository.'","controller":"gitrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"GitRepository","GitRepository":{"name":"flux-system","namespace":"flux-system"},"namespace":"flux-system","name":"flux-system","reconcileID":"4790be8b-f687-40be-a7c4-d465152a87f1"}
{"level":"info","ts":"2023-06-29T10:21:38.405Z","msg":"garbage collected 1 artifacts","controller":"gitrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"GitRepository","GitRepository":{"name":"flux-system","namespace":"flux-system"},"namespace":"flux-system","name":"flux-system","reconcileID":"ddc6d57d-61e1-4751-b65a-9b3f93c64c57"}
{"level":"info","ts":"2023-06-29T10:21:39.765Z","msg":"no changes since last reconcilation: observed revision 'main@sha1:65ca2c8bed62c5ea90cc944d6c24de3e52e95f67'","controller":"gitrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"GitRepository","GitRepository":{"name":"flux-system","namespace":"flux-system"},"namespace":"flux-system","name":"flux-system","reconcileID":"ddc6d57d-61e1-4751-b65a-9b3f93c64c57"}
```

```console
$ k logs deploy/source-controller-shard1 -n flux-system | tail -3
{"level":"info","ts":"2023-06-29T10:09:02.830Z","msg":"Starting workers","controller":"ocirepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"OCIRepository","worker count":2}
{"level":"info","ts":"2023-06-29T10:09:02.830Z","msg":"Starting workers","controller":"helmchart","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"HelmChart","worker count":2}
{"level":"info","ts":"2023-06-29T10:21:21.837Z","msg":"stored artifact for commit 'Sign v6.x release branch'","controller":"gitrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"GitRepository","GitRepository":{"name":"podinfo-shard1","namespace":"default"},"namespace":"default","name":"podinfo-shard1","reconcileID":"4cb87490-99ca-496c-a18c-151d518eabc5"}
```

This shows that the `source-controller-shard1` controller processed the sharded `GitRepository`.

### Adding an additional Shard

Update `clusters/my-cluster/demo/source-controller-shardset.yaml`

```yaml
apiVersion: templates.weave.works/v1alpha1
kind: FluxShardSet
metadata:
  name: source-controller-shardset
  namespace: flux-system
spec:
  sourceDeploymentRef:
    name: source-controller
  shards:
    - name: shard1
    - name: shard2
```

Add an additional `GitRepository` to `clusters/my-cluster/demo/gitrepository_with_shard2.yaml`

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: podinfo-shard2
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard2
spec:
  interval: 5m0s
  url: https://github.com/stefanprodan/podinfo
  ref:
    tag: "6.4.0"
```

You'll need to push this change

```sh
git add -A && git commit -m "Add second Shard and GitRepository." && git push
```

When the changes have been synced into the cluster:

```console
 $ kubectl get fluxshardsets -n flux-system
NAME                         READY   STATUS
source-controller-shardset   True    2 shard(s) created
```

The new controller has been created:

```console
$ kubectl get deploy -n flux-system
NAME                               READY   UP-TO-DATE   AVAILABLE   AGE
flux-sharding-controller-manager   1/1     1            1           98m
helm-controller                    1/1     1            1           147m
kustomize-controller               1/1     1            1           147m
notification-controller            1/1     1            1           147m
source-controller                  1/1     1            1           147m
source-controller-shard1           1/1     1            1           25m
source-controller-shard2           1/1     1            1           4m28s
```

And it's processing the `GitRepository` in shard2.

```console
$ k logs deploy/source-controller-shard2 -n flux-system | tail -3
{"level":"info","ts":"2023-06-29T10:29:54.212Z","msg":"Starting workers","controller":"helmrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"HelmRepository","worker count":2}
{"level":"info","ts":"2023-06-29T10:29:54.212Z","msg":"Starting workers","controller":"helmchart","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"HelmChart","worker count":2}
{"level":"info","ts":"2023-06-29T10:34:59.915Z","msg":"stored artifact for commit 'Merge pull request #273 from stefanprodan/release-...'","controller":"gitrepository","controllerGroup":"source.toolkit.fluxcd.io","controllerKind":"GitRepository","GitRepository":{"name":"podinfo-shard2","namespace":"default"},"namespace":"default","name":"podinfo-shard2","reconcileID":"63997b48-44f2-4368-8a91-f65428a46eb2"}
```

### Removing a Shard

You can remove shards by removing the Shard declaration:

Remove the file `clusters/my-cluster/demo/gitrepository_with_shard2.yaml` and edit the ShardSet:

```yaml
apiVersion: templates.weave.works/v1alpha1
kind: FluxShardSet
metadata:
  name: source-controller-shardset
  namespace: flux-system
spec:
  sourceDeploymentRef:
    name: source-controller
  shards:
    - name: shard1
```

After pushing the change:

```sh
git add -A && git commit -m "Remove second Shard and GitRepository." && git push
```

When this is reconciled into the cluster:

```console
$ kubectl get fluxshardsets -n flux-system
NAME                         READY   STATUS
source-controller-shardset   True    1 shard(s) created
$ kubectl get deploy -n flux-system
NAME                               READY   UP-TO-DATE   AVAILABLE   AGE
flux-sharding-controller-manager   1/1     1            1           103m
helm-controller                    1/1     1            1           152m
kustomize-controller               1/1     1            1           152m
notification-controller            1/1     1            1           152m
source-controller                  1/1     1            1           152m
source-controller-shard1           1/1     1            1           30m
```

The `shard2` controller is removed leaving just the deployment handling
`shard1`, this means that resources that have the label for `shard2` **will
not** be reconciled, both the `shard1` and default kustomize-controllers will
ignore resources for `shard2`.

## Upgrading the Flux controller

Changes to the controller referenced by `sourceDeploymentRef` are reflected into the managed shard controller, for example, when Flux is updated.