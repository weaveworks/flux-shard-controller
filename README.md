# flux-shard-controller

Easily spread load across replicated kustomize, source, helm and notification controllers

## Releasing

Publishing a **GitHub release** will trigger a GitHub Action to build and push the docker image and helm chart to ghcr.io.

### Create the release

Create a [new Github release](https://github.com/weaveworks/flux-shard-controller/releases/new)

1. Click "Choose a tag" and type in the tag that the release should create on publish (e.g. `v0.5.0`)
2. Click **Generate release notes**
3. Click **Publish release**

After a few minutes the new packages should be available to view via the repo's [packages page](https://github.com/orgs/weaveworks/packages?repo_name=flux-shard-controller).
