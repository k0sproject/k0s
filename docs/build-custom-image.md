<!--
SPDX-FileCopyrightText: 2026 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Build a custom k0s image

The root `Dockerfile` packages a Linux k0s binary and `docker-entrypoint.sh`
into an OCI image. It does not compile k0s. Use it to build an image containing
your own source changes or dependency updates.

The published images are built in this repository's
[release workflow](https://github.com/k0sproject/k0s/blob/main/.github/workflows/release.yml).
The workflow downloads the release binaries into architecture-specific
directories before building the image. The following steps prepare the same
directory layout locally.

## Prerequisites

- A checkout of the k0s repository, with your changes applied.
- Git, GNU Make, and a POSIX shell.
- Docker with BuildKit and a running Linux Docker daemon.
- Network access to download build images, dependencies, and Alpine packages.

Run the commands below from the repository root, in the same shell. The example
uses a native Linux AMD64 build. Use a fresh checkout for each architecture so
that previously built binaries and Make stamp files are not reused for a
different architecture. The default Make targets use Docker to build both k0s
and its embedded components, so a local Go installation is not required.

## Build the binary

On an AMD64 build host with an AMD64 Docker daemon, select:

```sh
TARGETARCH=amd64
PLATFORM=linux/amd64
```

For a native ARM64 build, use `TARGETARCH=arm64` and `PLATFORM=linux/arm64`.
For a native ARMv7 build, use `TARGETARCH=arm` and `PLATFORM=linux/arm/v7`.
These are the platforms used by the release image workflow. Keep the build
host, Docker daemon, and selected platform on the same architecture for this
procedure. Setting `TARGETARCH` or the image's `--platform` does not
cross-compile k0s or its embedded binaries.

Build k0s, including the binaries it needs at runtime:

```sh
make EMBEDDED_BINS_BUILDMODE=docker
```

This produces `./k0s`. Do not use `EMBEDDED_BINS_BUILDMODE=none` for this
image: that omits the embedded components. Stage the binary where the root
Dockerfile expects it:

```sh
mkdir -p "k0s-${TARGETARCH}"
cp k0s "k0s-${TARGETARCH}/k0s"
chmod +x "k0s-${TARGETARCH}/k0s"
```

For example, an AMD64 image requires `k0s-amd64/k0s` in the build context.
The directory name must match the binary's architecture.

## Build the image

Read the Alpine image reference from the repository, as the release workflow
does, and choose a local image tag:

```sh
ALPINE_IMAGE="$(./vars.sh alpine_image)"
IMAGE_TAG="$(./k0s version | tr '+' '-')-custom"
IMAGE="k0s-custom:${IMAGE_TAG}"

docker build --platform "$PLATFORM" \
  --build-arg ALPINE_IMAGE="$ALPINE_IMAGE" \
  --tag "$IMAGE" .
```

`ALPINE_IMAGE` is required because the Dockerfile has no default base image.
To use a different Alpine base, set this variable to an Alpine image reference
that supports your target platform. The Dockerfile installs `iptables` and
`tini` with `apk` and creates the service users needed by k0s.

BuildKit supplies `TARGETARCH` from `--platform`. The final `.` uses the
repository root as the build context, containing both `docker-entrypoint.sh`
and the staged binary. This command builds one architecture and tags the image
locally; it does not push it to a registry.

Docker tags cannot contain `+`. The command above replaces the `+` in the k0s
version with `-` and adds `-custom` to distinguish the local image. This does
not change the version embedded in the k0s binary.

Check the packaged binary and entrypoint help:

```sh
docker run --rm "$IMAGE" k0s version
docker run --rm "$IMAGE" --help
```

## Entrypoint configuration

The image runs `docker-entrypoint.sh` under `tini`. Its default command is
`k0s controller --enable-worker`. Pass a complete command after the image name
to override it, for example `k0s controller` for a controller without workers.

Set entrypoint variables at runtime with `docker run -e NAME=value`:

| Variable | Behavior |
| --- | --- |
| `K0S_CONFIG` | If nonempty, writes its contents to `/etc/k0s/config.yaml`. Ensure this path is writable and pass `k0s controller --enable-worker --config=/etc/k0s/config.yaml` to use it. |
| `K0S_ENTRYPOINT_REMOUNT_CGROUP2FS` | Set to `1` to force remounting the cgroup v2 filesystem read-write, or `0` to disable it. Otherwise, uses automatic detection. |
| `K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING` | Set to `1` to enable all available cgroup controllers, or `0` to disable nesting setup. Otherwise, uses automatic detection. |
| `K0S_ENTRYPOINT_DNS_FIXUP` | Set to `1` to apply the Docker embedded DNS fixup, or `0` to disable it. Otherwise, uses automatic detection. |
| `K0S_ENTRYPOINT_ROLE` | Overrides role detection from the command. Accepts `worker`, `controller`, or `controller+worker`. It controls entrypoint setup, not the k0s command being run. |
| `K0S_ENTRYPOINT_QUIET` | Set to `1` to suppress most entrypoint status messages. |

For controller-only or unrecognized commands, the entrypoint disables the
cgroup and DNS adjustments by default. Worker and controller-with-worker
commands use automatic detection. Explicit values override these defaults.
The image also sets `KUBECONFIG=/var/lib/k0s/pki/admin.conf` for kubectl.

## Run the custom image

Start a controller with worker components using the local tag:

```sh
docker run -d --name k0s-controller --hostname k0s-controller \
  -v /var/lib/k0s -v /var/log/pods \
  --tmpfs /run \
  --privileged \
  -p 6443:6443 \
  "$IMAGE"
```

See [Run k0s in Docker](k0s-in-docker.md) for explanations of the runtime
flags, controller-only operation, additional workers, cluster access, and
known limitations. Use your custom image tag in place of the published image
in those examples.
