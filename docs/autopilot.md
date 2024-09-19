# Autopilot

A tool for updating your `k0s` controller and worker nodes using specialized plans.
There is a public update-server hosted on the same domain as the documentation site. See the example below on how to use it. There is only a single channel `edge_release`  available. The channel exposes the latest  released version.

## How it works

* You create a `Plan` YAML
  * Defining the update payload (new version of `k0s`, URLs for platforms, etc)
  * Add definitions for all the nodes that should receive the update.
    * Either statically, or dynamically using label/field selectors
* Apply the `Plan`
  * Applying a `Plan` is a simple `kubectl apply` operation.
* Monitor the progress
  * The applied `Plan` provides a status that details the progress.

## Automatic updates

To enable automatic updates, create an `UpdateConfig` object:

```yaml
apiVersion: autopilot.k0sproject.io/v1beta2
kind: UpdateConfig
metadata:
  name: example
  namespace: default
spec:
  channel: edge_release
  updateServer: https://updates.k0sproject.io/
  upgradeStrategy:
    type: periodic
    periodic:
      # The folowing fields configures updates to happen only on Tue or Wed at 13:00-15:00
      days: [Tuesdsay,Wednesday]
      startTime: "13:00"
      length: 2h
  planSpec: # This defines the plan to be created IF there are updates available
    ...
```

## Safeguards

There are a number of safeguards in place to avoid breaking a cluster.

### Stateless Component

* The **autopilot** component were designed to not require any heavy state, or
massive synchronization. Controllers can disappear, and backup controllers can
resume the **autopilot** operations.

### Workers Update Only After Controllers

* The versioning that Kubelet and the Kubernetes API server adhere to requires that Kubelets
should **not** be of a newer version than the API server.

* How **autopilot** handles this is that when a `Plan` is applied that has both controller
and worker nodes, **all** of the controller nodes will be updated first. It is only when
**all** controllers have updated **successfully** that worker nodes will receive their
update instructions.

### Plans are Immutable

* When you apply a `Plan`, **autopilot** evaluates all of the controllers and workers that
should be included into the `Plan`, and tracks them in the status. After this point, no
additional changes to the plan (other than status) will be recognized.
  * This helps in largely dynamic worker node environments where nodes that may have been
    matched by the `selector` discovery method no longer exist by the time the update
    is ready to be scheduled.

### Controller Quorum Safety

* Prior to scheduling a controller update, **autopilot** queries the API server of **all**
  controllers to ensure that they report a successful `/ready`
* Only once all controllers are `/ready` will the current controller get sent update signaling.
* In the event that **any** controller reports a non-ready, the `Plan` transitions into an
  `InconsistentTargets` state, and the `Plan` execution ends.

### Controllers Update Sequentially

* Despite having the configuration options for controllers to set concurrency, only **one**
  controller will be updated at a time.

### Update Payload Verification

* Each `update` object payload can provide an optional `sha256` hash of the update content
  (specified in `url`), which is compared against the update content after it downloads.

## Configuration

**Autopilot** relies on a `Plan` object on its instructions on what to update.

Here is an arbitrary **Autopilot** plan:

```yaml
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot

spec:
  id: id1234
  timestamp: now

  commands:
    - k0supdate:
        version: v{{{ extra.k8s_version }}}+k0s.0
        platforms:
          linux-amd64:
            url: https://github.com/k0sproject/k0s/releases/download/v{{{ extra.k8s_version }}}+k0s.0/k0s-v{{{ extra.k8s_version }}}+k0s.0-amd64
            sha256: '0000000000000000000000000000000000000000000000000000000000000000'
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - ip-172-31-44-131
                  - ip-172-31-42-134
                  - ip-172-31-39-65
          workers:
            limits:
              concurrent: 5
            discovery:
              selector:
                labels: environment=staging
                fields: metadata.name=worker2
```

### Core Fields

#### `apiVersion <string> (required)`

* The current version of the Autopilot API is `v1beta2`, with a full group-version of `autopilot.k0sproject.io/v1beta2`

#### `metadata.name <string> (required)`

* The name of the plan should always be `autopilot`
  * **Note:** Plans will not execute if they don't follow this convention.

### Spec Fields

#### `spec.id <string> (optional)`

* An identifier that can be provided by the creator for informational and tracking purposes.

#### `spec.timestamp <string> (optional)`

* A timestamp value that can be provided by the creator for informational purposes. **Autopilot**
does nothing with this information.

#### `spec.commands[] (required)`

* The `commands` contains all of the commands that should be performed as a part of the plan.

### **`k0supdate`** Command

#### `spec.commands[].k0supdate.version <string> (required)`

* The version of the binary being updated. This version is used to compare against the installed
version before and after update to ensure success.

#### `spec.commands[].k0supdate.platforms.*.url <string> (required)`

* An URL providing where the updated binary should be downloaded from, for this specific platform.
  * The naming of platforms is a combination of `$GOOS` and `$GOARCH`, separated by a hyphen (`-`)
    * eg: `linux-amd64`, `linux-arm64`, `linux-arm`
  * **Note:** The main supported platform is `linux`. **Autopilot** may work on other platforms, however
this has not been tested.

#### `spec.commands[].k0supdate.platforms.*.sha256 <string> (optional)`

* If a SHA256 hash is provided for the binary, the completed downloaded will be verified against it.

#### `spec.commands[].k0supdate.targets.controllers <object> (optional)`

* This object provides the details of how `controllers` should be updated.

#### `spec.commands[].k0supdate.targets.controllers.limits.concurrent <int> (fixed as 1)`

* The configuration allows for specifying the number of concurrent controller updates
through the plan spec, however for controller targets this is fixed always to `1`.
* By ensuring that only one controller updates at a time, we aim to avoid scenarios
where quorom may be disrupted.

#### `spec.commands[].k0supdate.targets.workers <object> (optional)`

* This object provides the details of how `workers` should be updated.

#### `spec.commands[].k0supdate.targets.workers.limits.concurrent <int> (optional, default = 1)`

* Specifying a `concurrent` value for worker targets will allow for that number of workers
to be updated at a time. If no value is provided, `1` is assumed.

### **`airgapupdate`** Command

#### `spec.commands[].airgapupdate.version <string> (required)`

* The version of the airgap bundle being updated.

#### `spec.commands[].airgapupdate.platforms.*.url <string> (required)`

* An URL providing where the updated binary should be downloaded from, for this specific platform.
  * The naming of platforms is a combination of `$GOOS` and `$GOARCH`, separated by a hyphen (`-`)
    * eg: `linux-amd64`, `linux-arm64`, `linux-arm`
  * **Note:** The main supported platform is `linux`. **Autopilot** may work on other platforms, however
this has not been tested.

#### `spec.commands[].airgapupdate.platforms.*.sha256 <string> (optional)`

* If a SHA256 hash is provided for the binary, the completed downloaded will be verified against it.

#### `spec.commands[].airgapupdate.workers <object> (optional)`

* This object provides the details of how `workers` should be updated.

#### `spec.commands[].airgapupdate.workers.limits.concurrent <int> (optional, default = 1)`

* Specifying a `concurrent` value for worker targets will allow for that number of workers
to be updated at a time. If no value is provided, `1` is assumed.

### Static Discovery

This defines the `static` discovery method used for this set of targets (`controllers`, `workers`). The `static` discovery method relies on a fixed set of hostnames defined
in `.nodes`.

It is expected that a `Node` (workers) or `ControlNode` (controllers) object exists with
the same name.

```yaml
  static:
    nodes:
      - ip-172-31-44-131
      - ip-172-31-42-134
      - ip-172-31-39-65
```

#### `spec.commands[].k0supdate.targets.*.discovery.static.nodes[] <string> (required for static)`

* A list of hostnames that should be included in target set (`controllers`, `workers`).

### Selector Target Discovery

The `selector` target discovery method relies on a dynamic query to the Kubernetes API
using labels and fields to produce a set of hosts that should be updated.

Providing both `labels` and `fields` in the `selector` definition will result in a logical `AND` of both operands.

```yaml
  selector:
    labels: environment=staging
    fields: metadata.name=worker2
```

Specifying an empty selector will result in *all* nodes being selected for this target set.

```yaml
  selector: {}
```

#### `spec.commands[].k0supdate.targets.*.discovery.selector.labels <string> (optional)`

* A collection of name/value labels that should be used for finding the appropriate nodes
for the update of this target set.

#### `spec.commands[].k0supdate.targets.*.discovery.selector.fields <string> (optional)`

* A collection of name/value fields that should be used for finding the appropriate nodes
for the update of this target set.
  * **Note:** Currently only the field `metadata.name` is available as a query field.

## Status Reporting

After a `Plan` has been applied, its progress can be viewed in the `.status` of the
`autopilot` Plan.

```shell
    kubectl get plan autopilot -oyaml
```

An example of a `Plan` status:

```yaml
  status:
    state: SchedulableWait
    commands:
    - state: SchedulableWait
      k0supdate:
        controllers:
        - lastUpdatedTimestamp: "2022-04-07T15:52:44Z"
          name: controller0
          state: SignalCompleted
        - lastUpdatedTimestamp: "2022-04-07T15:52:24Z"
          name: controller1
          state: SignalCompleted
        - lastUpdatedTimestamp: "2022-04-07T15:52:24Z"
          name: controller2
          state: SignalPending
        workers:
        - lastUpdatedTimestamp: "2022-04-07T15:52:24Z"
          name: worker0
          state: SignalPending
        - lastUpdatedTimestamp: "2022-04-07T15:52:24Z"
          name: worker1
          state: SignalPending
        - lastUpdatedTimestamp: "2022-04-07T15:52:24Z"
          name: worker2
          state: SignalPending
```

To read this status, this indicates that:

* The overall status of the update is `SchedulableWait`, meaning that **autopilot** is
  waiting for the next opportunity to process a command.
* There are three controller nodes
  * Two controllers have `SignalCompleted` successfully
  * One is waiting to be signalled (`SignalPending`)
* There are also three worker nodes
  * All are awaiting signaling updates (`SignalPending`)

### Plan Status

The `Plan` status at `.status.status` represents the overall status of the **autopilot**
update operation. There are a number of statuses available:

| Status | Description | Ends Plan? |
| ------ | ----------- | ---------- |
| `IncompleteTargets` | There are nodes in the resolved `Plan` that do not have associated `Node` (worker) or `ControlNode` (controller) objects. | Yes |
| `InconsistentTargets` | A controller has reported itself as not-ready during the selection of the next controller to update. | Yes |
| `Schedulable` | Indicates that the `Plan` can be re-evaluated to determine which next node to update. | No |
| `SchedulableWait` | Scheduling operations are in progress, and no further update scheduling should occur. | No |
| `Completed` | The `Plan` has run successfully to completion. | Yes |
| `Restricted` | The `Plan` included node types (controller or worker) that violates the `--exclude-from-plans` restrictions. | Yes |

### Node Status

Similar to the **Plan Status**, the individual nodes can have their own statuses:

| Status | Description |
| ------ | ----------- |
| `SignalPending` | The node is available and awaiting an update signal |
| `SignalSent` | Update signaling has been successfully applied to this node. |
| `MissingPlatform` | This node is a platform that an update has not been provided for. |
| `MissingSignalNode` | This node does have an associated `Node` (worker) or `ControlNode` (controller) object. |

## UpdateConfig

### UpdateConfig Core Fields

#### `apiVersion <string> (required field)`

* API version. The current version of the Autopilot API is `v1beta2`, with a full group-version of `autopilot.k0sproject.io/v1beta2`

#### `metadata.name <string> (required field)`

* Name of the config.

### Spec

#### `spec.channel <string> (optional)`

* Update channel to use. Supported values: `stable`(default), `unstable`.

#### `spec.updateServer <string> (optional)`

* Update server url. Defaults to `https://updates.k0sproject.io`

#### `spec.upgradeStrategy.type <enum:cron|periodic>`

* Select which update strategy to use.

#### `spec.upgradeStrategy.cron <string> (optional)` **DEPRECATED**

* Schedule to check for updates in crontab format.

#### `spec.upgradeStrategy.cron <object>`

Fields:

* `days`: On which weekdays to check for updates
* `startTime`: At which time of day to check updates
* `length`: The length of the update window

#### `spec.planSpec <string> (optional)`

* Describes the behavior of the autopilot generated `Plan`

### Example

```yaml
apiVersion: autopilot.k0sproject.io/v1beta2
kind: UpdaterConfig
metadata:
  name: example
spec:
  channel: stable
  updateServer: https://updates.k0sproject.io/
  upgradeStrategy:
    type: periodic
    periodic:
      # The folowing fields configures updates to happen only on Tue or Wed at 13:00-15:00
      days: [Tuesdsay,Wednesday]
      startTime: "13:00"
      length: 2h
  # Optional. Specifies a created Plan object
  planSpec:
    commands:
      - k0supdate: # optional
          forceupdate: true # optional
          targets:
            controllers:
              discovery:
                static:
                  nodes:
                    - ip-172-31-44-131
                    - ip-172-31-42-134
                    - ip-172-31-39-65
            workers:
              limits:
                concurrent: 5
              discovery:
                selector:
                  labels: environment=staging
                  fields: metadata.name=worker2
        airgapupdate: # optional
          workers:
            limits:
              concurrent: 5
            discovery:
              selector:
                labels: environment=staging
                fields: metadata.name=worker2
```

## FAQ

### Q: How do I apply the `Plan` and `ControlNode` CRDs?

A: These CRD definitions are embedded in the **k0s** binary and applied on startup.
No additional action is needed.

### Q: How will `ControlNode` instances get removed?

A: `ControlNode` instances are created by **autopilot** controllers as they startup. When
controllers disappear, they will **not** remove their associated `ControlNode` instance. It
is the responsibility of the operator/administrator to ensure their maintenance.

### Q: I upgraded my workers, and now Kubelets are no longer reporting

You probably upgraded your workers to an API version greater than what is
available on the API server.

https://kubernetes.io/releases/version-skew-policy/

> Make sure that your controllers are at the desired version **first** before
> upgrading workers.
