# Custom CRI runtime

k0s supports users bringing their own CRI runtime (for example, docker). In which case, k0s will not start nor manage the runtime, and it is fully up to the user to configure it properly.

To run a k0s worker with a custom CRI runtime use the option `--cri-socket`. 
It takes input in the form of `<type>:<socket>` where:

- `type`: Either `remote` or `docker`. Use `docker` for pure docker setup, `remote` for anything else.
- `socket`: Path to the socket, examples: `unix:///var/run/docker.sock`

To run k0s with pre-existing docker setup run the worker with `k0s worker --cri-socket docker:unix:///var/run/docker.sock <token>`.

When `docker` is used as a runtime, k0s will configure kubelet to create the dockershim socket at `/var/run/dockershim.sock`.