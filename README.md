# KubeStellar Workload Stager

A Kubernetes application that utilizes KubeStellar mechanisms for multi-cluster workload staged placement.
The application consists of a new community API CRD - StagedBindingPolicy - and a controller that reconciles the CRD instances.

## StagedBindingPolicy Resource

A StagedBindingPolicy introduces the concept of staged placement - where each stage defines a BindingPolicy
and a condition (CEL) that must be satisfied for the stage to be considered fulfilled.

```yaml
TODO
   ```

As visible in the sample CR, a stage consists of:
- A BindingPolicySpec
- A CEL expression to filter out **DownsyncClauses** (workload object identifiers) that are selected in the Binding generated by the BindingPolicy
- A CEL expression condition that is checked against CombinedStatuses associated with the filtered DownsyncClauses
  - Currently the single expression is checked against all the relevant CombinedStatuses

A sample use of the StagedBindingPolicy is to juggle a workload between dev-test-prod environments,
where for each environment a stage would be defined with a relevant completion condition.

## StagedBindingPolicy Controller

When a StagedBindingPolicy is created, the controller creates a new BindingPolicy as specified in the first stage. 
The controller watches Bindings and CombinedStatuses in order to determine when an active stage is fulfilled. 
When a stage is fulfilled, the controller increments the stage (saved in the StagedBindingPolicy status) and creates
a new BindingPolicy with the next stage.
