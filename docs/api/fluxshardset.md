<p>Packages:</p>
<ul>
<li>
<a href="#templates.weave.works%2fv1alpha1">templates.weave.works/v1alpha1</a>
</li>
</ul>
<h2 id="templates.weave.works/v1alpha1">templates.weave.works/v1alpha1</h2>
<p>Package v1alpha1 contains API Schema definitions for the gitopssets v1alpha1 API group</p>
Resource Types:
<ul></ul>
<h3 id="templates.weave.works/v1alpha1.FluxShardSet">FluxShardSet
</h3>
<p>FluxShardSet is the Schema for the fluxshardsets API</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br />
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.FluxShardSetSpec">
FluxShardSetSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tbody>
<tr>
<td>
<code>suspend</code><br />
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend tells the controller to suspend the reconciliation of this
FluxShardSet.</p>
</td>
</tr>
<tr>
<td>
<code>sourceDeploymentRef</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.SourceDeploymentReference">
SourceDeploymentReference
</a>
</em>
</td>
<td>
<p>Reference the source Deployment.</p>
</td>
</tr>
<tr>
<td>
<code>shards</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.ShardSpec">
[]ShardSpec
</a>
</em>
</td>
<td>
<p>Shards is a list of shards to deploy</p>
</td>
</tr>
</tbody>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.FluxShardSetStatus">
FluxShardSetStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="templates.weave.works/v1alpha1.FluxShardSetSpec">FluxShardSetSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#templates.weave.works/v1alpha1.FluxShardSet">FluxShardSet</a>)
</p>
<p>FluxShardSetSpec defines the desired state of FluxShardSet</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>suspend</code><br />
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend tells the controller to suspend the reconciliation of this
FluxShardSet.</p>
</td>
</tr>
<tr>
<td>
<code>sourceDeploymentRef</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.SourceDeploymentReference">
SourceDeploymentReference
</a>
</em>
</td>
<td>
<p>Reference the source Deployment.</p>
</td>
</tr>
<tr>
<td>
<code>shards</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.ShardSpec">
[]ShardSpec
</a>
</em>
</td>
<td>
<p>Shards is a list of shards to deploy</p>
</td>
</tr>
</tbody>
</table>
<h3 id="templates.weave.works/v1alpha1.FluxShardSetStatus">FluxShardSetStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#templates.weave.works/v1alpha1.FluxShardSet">FluxShardSet</a>)
</p>
<p>FluxShardSetStatus defines the observed state of FluxShardSet</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ReconcileRequestStatus</code><br />
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#ReconcileRequestStatus">
github.com/fluxcd/pkg/apis/meta.ReconcileRequestStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReconcileRequestStatus</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br />
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last observed generation of the HelmRepository
object.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br />
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions holds the conditions for the FluxShardSet</p>
</td>
</tr>
<tr>
<td>
<code>inventory</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.ResourceInventory">
ResourceInventory
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Inventory contains the list of Kubernetes resource object references that
have been successfully applied</p>
</td>
</tr>
</tbody>
</table>
<h3 id="templates.weave.works/v1alpha1.ResourceInventory">ResourceInventory
</h3>
<p>
(<em>Appears on:</em>
<a href="#templates.weave.works/v1alpha1.FluxShardSetStatus">FluxShardSetStatus</a>)
</p>
<p>ResourceInventory contains a list of Kubernetes resource object references that have been created for the Shard Set.</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>entries</code><br />
<em>
<a href="#templates.weave.works/v1alpha1.ResourceRef">
[]ResourceRef
</a>
</em>
</td>
<td>
<p>Entries of Kubernetes resource object references.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="templates.weave.works/v1alpha1.ResourceRef">ResourceRef
</h3>
<p>
(<em>Appears on:</em>
<a href="#templates.weave.works/v1alpha1.ResourceInventory">ResourceInventory</a>)
</p>
<p>ResourceRef contains the information necessary to locate a resource within a cluster.</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code><br />
<em>
string
</em>
</td>
<td>
<p>ID is the string representation of the Kubernetes resource object&rsquo;s metadata,
in the format &lsquo;namespace_name_group_kind&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>v</code><br />
<em>
string
</em>
</td>
<td>
<p>Version is the API version of the Kubernetes resource object&rsquo;s kind.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="templates.weave.works/v1alpha1.ShardSpec">ShardSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#templates.weave.works/v1alpha1.FluxShardSetSpec">FluxShardSetSpec</a>)
</p>
<p>ShardSpec defines a shard to deploy</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br />
<em>
string
</em>
</td>
<td>
<p>Name is the name of the shard</p>
</td>
</tr>
</tbody>
</table>
<h3 id="templates.weave.works/v1alpha1.SourceDeploymentReference">SourceDeploymentReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#templates.weave.works/v1alpha1.FluxShardSetSpec">FluxShardSetSpec</a>)
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br />
<em>
string
</em>
</td>
<td>
<p>Name of the referent.</p>
</td>
</tr>
</tbody>
</table>
<div>
<p>This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
