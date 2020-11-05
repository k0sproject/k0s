# Custom CRI runtime

k0s supports users bringing their own CRI runtime such as Docker. In this case k0s will not start nor manage the runtime, it's fully up to the user to configure it properly.

To run k0s worker with a custom CRI runtime use the option `--cri-socket`. It takes input in the form of `<type>:<socket>` where:
- `type`: Either `remote` or `docker`. Use `docker` for pure docker setup, remote for everything else.
- `socket`: Path to the socket, examples: `unix:///var/run/docker.sock`

To run k0s with pre-existing docker setup run the worker with `k0s worker --cri-socket docker:unix:///var/run/docker.sock <token>`.

In case docker is used k0s will configure kubelet to create the dockershim socket at `/var/run/dockershim.sock`.