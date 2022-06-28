# Autopilot: Developer Reference
**Autopilot** is currently a tool that runs alongside `k0s` controllers and workers, responsible for applying `k0s` updates.

# Architecture
**Autopilot** is built on top of the https://github.com/kubernetes-sigs/controller-runtime project, allowing for easy management of Kubernetes
objects.

## Controllers + Workers
**Autopilot** can run in either a `controller` mode or `worker` mode.

> It is required that **autopilot** be configured to run in the same mode
  as the `k0s` node it is running beside.
>
>(ie. **autopilot** worker runs beside a `k0s` worker node).

### Controllers
* Controllers have the special functionality of coordinating with themselves,
determining a 'leader'. The leader has the responsibility to perform all of
the **autopilot** functionality for the cluster.
* If the leader is lost, one of the other controllers will get promoted to
the leader and resume operations.
* Controllers also act as 'workers' for themselves (only) when they receive
signaling updates.

### Workers
* Workers are only interested in receiving signaling updates from controllers.
* They only perform operations on the `k0s` node they are running beside.

## Signal Nodes
From a high-level, **autopilot** is only interested updating things that are
logically known as "signal nodes". Signal nodes effectively map to
a Kubernetes object type. There are two types of signal nodes
available:
* `ControlNode`: This is an autopilot-specific CRD for nodes that run on controllers.
* `Node`: This is the Kubernetes `Node` object.

## Signaling
Signaling is the act of adding special annotation metadata to a signal node
in order to drive the **autopilot** update process.

The communication between **controller** and **node** instances will be done via Kubernetes
Annotations using embedded JSON structures. Effectively, the **autopilot** controller promises
to write a specific set of annotations per-node, and the **autopilot** instance running on the
node promises to update the JSON structure with its own specific responses.

This JSON format enforces that this data is considered internal to **autopilot**, and can change
as versions diverge.

### v2
The **autopilot** signaling protocol `v2` uses an embedded JSON structure in Kubernetes Annotations.
The detailing of the structure is intentionally omitted as this can change at any time.

### Directives
The following outlines the various directives that can be used in **autopilot** signalling.

> All directives are required unless explicitly stated as optional.

`autopilot-signal-version`
* The version of the **autopilot** protocol as defined by the **autopilot** controller.  This
is a string value that does not mandate any version-specific format.

`autopilot-signal-data`
* The encoded JSON object representing the state of the nodes signaling.


## Plans
A `Plan` is a YAML document that describes what is going to be update,
and who will be receiving the update.

See [README.md](README.md) for a breakdown of what exists in a `Plan`.

## Roots
There are two logical "roots", one for running with `k0s
controllers`, and one for running with `k0s workers`. Each root is responsible
for running a specific set of custom `controller-runtime` controllers, handling
the **autopilot** functionality.

## Root: For k0s controllers (aka: the "controller" option)
The `root_controller.go` root is what is run when **autopilot** is started with
the `"controller"` argument.
  * This has special logic that ensures that only **one** **autopilot** controller
    instance is "active", aka the `leader`
  * All other **autopilot** controllers run in a `follower` mode when not active.
  * As leadership is lost, other **autopilot** controllers will take over the role.
  * All **autopilot** controllers start in a `follower` mode, until granted leadership.

## Root: For k0s workers (aka: the "worker" option)
The `root_worker.go` root is what is run when **autopilot** is started with the
`"worker"` argument.
* **Autopilot** workers only listen for signaling notifications for instructions
  on how to update.

# Controllers
The controllers that get run at **autopilot** startup are determined by the mode
autopilot runs in (`"controller"` or `"worker"`). In addition, specific phases
of an **autopilot** update will also leverage different controllers. Each is
responsible for performing **one** operation.

## Plan Controllers
Plan controllers are controllers that pay attention to the state + status of
the current plan, and help move the plan to completion.

| Controller | Description |
| ---------- | ----------- |
| `newplan`  | Processes a new `Plan` by validating its content, and creating the appropriate `.Status` fields for each node specified in the `Plan`. |
| `schedulablewait` | Determines if any of the specified signal nodes can receive an update. |
| `schedulable` | Performs the scheduling of an update for a signal node by adding signaling data to the objects annotations. |
| `complete` | Not a "controller", but a state that plans are put into via the core plan controllers. |

The `schedulablewait` and `schedulable` controllers will routinely move a plan to/from these status. This is observed
for controllers, where `schedulablewait` ensures that only one signal node can be in the middle of an update at a time.

    newplan --> schedulablewait <--+--> schedulable
                                    \
                                     +--> complete


## Signal Controllers
Signal Controllers are controllers that take action against the signaling
annotation metadata that is applied against a signal node.

All of these signal controllers are in the context of a
`k0s` update command.

| Controller | Description |
| ---------- | ----------- |
| `signal`   | Performs the initial inspection of signaling annotation data on signal nodes, in order to start the requested operation (currently only `"update"`)|
| `download` | Downloads + verifies the update data.|
| `cordon`   | Cordons and drains a worker node. |
| `apply`    | Performs all of the required operations to apply the update data to the host/destination. |
| `restart`  | Restarts `k0s` + requeues until is is available again. |
| `uncordon` | Un-cordons a worker node. |
| `complete` | Updates the status for this signal node in the main plan status. |

The transition between the signal node events follows the table.

# Development

## Building
    make build

## Testing
    make test

## Integration Tests
    make inttest

## Updating the generated CRD code
The generation is handled by a script called `crd-gen.sh` which runs both `controller-gen`
and `client-gen` in containers, outputting to the local tree.

    ./hack/crd-gen/crd-gen.sh autopilot autopilot.k0sproject.io v1beta2

