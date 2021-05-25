# Custom CRI runtime

**Warning**: You can use your own CRI runtime with k0s (for example, `docker`), however k0s will not start or manage the runtime, and configuration is solely your responsibility.

Use the option `--cri-socket` to run a k0s worker with a custom CRI runtime. the option takes input in the form of `<type>:<socket_path>` (for `type`, use `docker` for a pure Docker setup and `remote` for anything else).

To run k0s with a pre-existing Docker setup, run the worker with `k0s worker --cri-socket docker:unix:///var/run/docker.sock <token>`.

When `docker` is used as a runtime, k0s configures kubelet to create the dockershim socket at `/var/run/dockershim.sock`.