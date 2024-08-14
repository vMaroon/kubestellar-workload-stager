# KubeStellar Workload Stager

A Kubernetes application that utilizes KubeStellar mechanisms for multi-cluster workload staged placement.
The application consists of a new community API CRD - StagedBindingPolicy - and a controller that reconciles the CRD instances.

## StagedBindingPolicy Resource

A StagedBindingPolicy introduces the concept of staged placement - where each stage defines a BindingPolicy
and a condition (CEL) that must be satisfied for the stage to be considered fulfilled.

### Spec

```yaml
apiVersion: community.kubestellar.io/v1alpha1
kind: StagedBindingPolicy
metadata:
  name: nginx-staged-bindingpolicy
spec:
  downsync:
    - statusCollectors:
      - deployments-aggregator
      objectSelectors:
      - matchLabels: {"app.kubernetes.io/name":"nginx"}
  stages:
    - name: dev
      clusterSelectors:
      - matchLabels: {"env":"dev"}
      filter: downsyncClause.resource == "deployments"
      condition: obj.results.exists(r, r.name == 'deployments-aggregator' && r.rows.exists(row, row.columns.exists(col, col.type == 'Number' && col.float == '1')))
    - name: prod
      clusterSelectors:
      - matchLabels: {"env":"prod"}
   ```

Breaking down the StagedBindingPolicy resource:
- `downsync` - a list of `what` selectors identical to those in the BindingPolicy API.
This list defines the workload objects to be synced in all stages.
- `stages` - a list of stages, each with the following fields:
  - `name` - the name of the stage.
  - `clusterSelectors` - a list of `where` selectors identical to those in the BindingPolicy API.
  This list defines the clusters to which the stage applies.
  - `filter` - a CEL expression that filters the objects to be synced in the stage. 
The expression's root is `downsyncClause` which is a [DownsyncObject](https://github.com/kubestellar/kubestellar/blob/93e0bf91b99a5978ba54cece928292fb69a50e46/api/control/v1alpha1/types.go#L298) as appears in the Binding API.
  - `condition` - a CEL expression that defines the stage completion condition.
The expression's root is `obj` which is a [CombinedStatus](https://github.com/kubestellar/kubestellar/blob/93e0bf91b99a5978ba54cece928292fb69a50e46/api/control/v1alpha1/types.go#L488)

The filter filters out workload objects that are selected by the `downsync` clause - as determined in the generated Binding.
The condition is evaluated on the CombinedStatuses associated with the filtered workload objects.

### Status

```yaml
status:
  activeStage: <name>
```

The status of the StagedBindingPolicy resource includes the name of the active stage.

## StagedBindingPolicy Controller

When a StagedBindingPolicy is created, the controller creates a new BindingPolicy as specified in the first stage. 
The controller watches Bindings and CombinedStatuses in order to determine when an active stage is fulfilled. 
When a stage is fulfilled, the controller increments the stage (saved in the StagedBindingPolicy status) and creates
a new BindingPolicy with the next stage.

## StagedBindingPolicy Use Case
A sample use of the StagedBindingPolicy is to juggle a workload between dev-test-prod environments,
where for each environment a stage would be defined with a relevant completion condition.
