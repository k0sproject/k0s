#!/bin/sh
# SPDX-FileCopyrightText: 2020, 2024 k0s authors
# SPDX-License-Identifier: Apache-2.0
#shellcheck disable=SC3040,SC3043,SC3052

set -euo pipefail

usage() {
  cat <<EOF
Usage: $0 ARGS...

The container entry point script for k0s. It sets the stage for k0s to work in
a containerized environment. This includes possible cgroup and iptables rule
customizations.

Arguments:
  help, -h, --help    Print this help message and exit

Environment variables:
  K0S_CONFIG
    Optional configuration for k0s, written to /etc/k0s/config.yaml if set.

  K0S_ENTRYPOINT_REMOUNT_CGROUP2FS
    Set to 1 to force remounting of the cgroup2 filesystem in read-write mode.
    Set to 0 to disable remounting.
    The default is automatic detection.

  K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING
    Set to 1 to always enable all available cgroup controllers for the root cgroup.
    Set to 0 to disable cgroup nesting.
    The default is automatic detection.

  K0S_ENTRYPOINT_DNS_FIXUP
    Set to 1 to apply DNS fixes required for the Docker embedded DNS setup.
    Set to 0 to disable.
    Default is automatic detection.

  K0S_ENTRYPOINT_ROLE
    Specifies the role for k0s. Can be 'worker', 'controller', or 'controller+worker'.
    Default is to autodetect the role from the arguments.
    Depending on the role, some of the above features will be disabled by default.

  K0S_ENTRYPOINT_QUIET
    Set to 1 to suppress printing status messages.

EOF
}

# Get the effective process capabilities.
get_effective_capabilities() {
  local key val
  while read -r key val; do
    if [ "$key" = CapEff: ]; then
      echo $((16#$val))
      return
    fi
  done </proc/self/status
  return 1
}

has_effective_capability() {
  local cap_eff
  cap_eff=$(get_effective_capabilities) || return 2
  # Check if the requested bit is set.
  [ "$((cap_eff & $((1 << $1))))" != 0 ]
}

# Checks if this process has CAP_NET_ADMIN.
has_cap_net_admin() {
  # CAP_NET_ADMIN is bit 12 (https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/include/uapi/linux/capability.h?h=v3.10#n188)
  has_effective_capability 12
}

# Checks if this process has CAP_SYS_ADMIN.
has_cap_sys_admin() {
  # CAP_SYS_ADMIN is bit 21 (https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/include/uapi/linux/capability.h?h=v3.10#n263)
  has_effective_capability 21
}

# Checks if the cgroup2 file system is mounted at its well-known path.
is_cgroupv2() {
  # Check for the magic number of the cgroup2 fs.
  # https://www.kernel.org/doc/html/v5.16/admin-guide/cgroup-v2.html#mounting
  [ "$(stat -f -c %t /sys/fs/cgroup)" = 63677270 ]
}

# Checks if the file system mounted at the well-known cgroup2 path is read-write.
is_cgroupfs_rw() {
  local _device mountpoint _fstype opts _rest
  while read -r _device mountpoint _fstype opts _rest; do
    if [ "$mountpoint" = /sys/fs/cgroup ]; then
      case "$opts" in
      rw* | *,rw | *,rw,*) return 0 ;;
      esac
      break
    fi
  done </proc/mounts
  return 1
}

# Remounts the cgroup2 file system in read-write mode, if necessary.
remount_cgroup2fs() {
  case "${K0S_ENTRYPOINT_REMOUNT_CGROUP2FS-}" in
  0) return ;; # disabled
  1) ;;        # enabled
  *)           # auto detect
    if ! is_cgroupv2 || is_cgroupfs_rw; then
      return
    fi
    has_cap_sys_admin || {
      if [ $? -eq 1 ]; then
        echo "$0: won't remount /sys/fs/cgroup without CAP_SYS_ADMIN (disable with K0S_ENTRYPOINT_REMOUNT_CGROUP2FS=0)" >&2
        return
      fi

      echo "$0: failed to determine capabilities (disable with K0S_ENTRYPOINT_REMOUNT_CGROUP2FS=0)" >&2
    }
    ;;
  esac

  mount --make-rslave / # don't propagate mount events to other namespaces
  mount -o remount,rw /sys/fs/cgroup

  [ "${K0S_ENTRYPOINT_QUIET-}" = '1' ] || echo "$0: remounted /sys/fs/cgroup" >&2
}

get_process_cgroupv2() {
  local cg
  while read -r cg; do
    case "$cg" in
    # The entry for cgroup v2 is always in the format "0::$PATH"
    # https://www.kernel.org/doc/html/v5.16/admin-guide/cgroup-v2.html#processes
    0::/*) echo "${cg#0::*}" && return 0 ;;
    *) cg='' ;;
    esac
  done </proc/self/cgroup
  return 1
}

# Enables all available controllers for the root cgroup.
enable_cgroupv2_nesting() {
  case "${K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING-}" in
  0) return ;;        # disabled
  1) local force=1 ;; # enabled
  *) local force=0 ;; # auto detect
  esac

  [ $force = 1 ] || is_cgroupv2 || return

  local cg
  cg="$(get_process_cgroupv2)" || {
    echo "$0: failed to determine process cgroup (disable with K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING=0)" >&2
    return 1
  }
  local cg_path=/sys/fs/cgroup"$cg"

  local all_controllers
  read -r all_controllers <"$cg_path"/cgroup.controllers || {
    echo "$0: failed to load available controllers for cgroup $cg (disable with K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING=0)" >&2
    return 1
  }

  if [ $force != 1 ]; then
    local enabled_controllers
    read -r enabled_controllers <"$cg_path"/cgroup.subtree_control || true # file may be empty
    [ "$all_controllers" != "$enabled_controllers" ] || return

    is_cgroupfs_rw || {
      echo "$0: won't enable all available cgroup controllers for cgroup $cg as the cgroup2 file system is read-only (disable with K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING=0)" >&2
      return
    }
  fi

  # move all processes out of the root cgroup, otherwise the controllers can't be enabled
  if [ "$cg" = / ]; then
    mkdir -p /sys/fs/cgroup/entrypoint.scope
    local pid
    while read -r pid; do
      echo "$pid" >/sys/fs/cgroup/entrypoint.scope/cgroup.procs
    done </sys/fs/cgroup/cgroup.procs
  fi

  # enable all available controllers
  set --
  local controller
  for controller in $all_controllers; do set -- "$@" +"$controller"; done
  echo "$@" >/sys/fs/cgroup/cgroup.subtree_control
  [ "${K0S_ENTRYPOINT_QUIET-}" = '1' ] || {
    echo "$0: enabled all available controllers for cgroup $cg: $all_controllers" >&2
  }
}

# DNS fixup adapted from kind.
dns_fixup() {
  case "${K0S_ENTRYPOINT_DNS_FIXUP-}" in
  0) return ;; # disabled
  1) ;;        # enabled
  *)           # auto detect
    has_cap_net_admin || {
      if [ $? -eq 1 ]; then
        echo "$0: won't apply DNS fixes without CAP_NET_ADMIN (disable with K0S_ENTRYPOINT_DNS_FIXUP=0)" >&2
        return
      fi

      echo "$0: failed to determine capabilities (disable with K0S_ENTRYPOINT_DNS_FIXUP=0)" >&2
    } ;;
  esac

  # SPDX-SnippetBegin
  # SPDX-License-Identifier: Apache-2.0
  # SPDX-SnippetCopyrightText: 2019 The Kubernetes Authors.
  # SPDX-SnippetCopyrightText: 2020 the k0s authors
  # SDPX—SnippetName: Modified parts of the enable_network_magic function from kind's entrypoint script
  # SPDX-SnippetComment: https://github.com/kubernetes-sigs/kind/blob/7568bf728147c1253e651f25edfd0e0a75534b8a/images/base/files/usr/local/bin/entrypoint#L447-L487

  local docker_embedded_dns_ip docker_host_ip iptables

  # well-known docker embedded DNS is at 127.0.0.11:53
  docker_embedded_dns_ip=127.0.0.11

  # first we need to detect an IP to use for reaching the docker host
  docker_host_ip=$(timeout 5 getent ahostsv4 host.docker.internal | head -n1 | cut -d' ' -f1 || true)
  # if the ip doesn't exist or is a loopback address use the default gateway
  case "$docker_host_ip" in
  '' | 127.*) docker_host_ip=$(ip -4 route show default | cut -d' ' -f3) ;;
  esac

  for iptables in iptables iptables-nft; do
    # patch docker's iptables rules to switch out the DNS IP
    "$iptables"-save \
      | sed \
        `# switch docker DNS DNAT rules to our chosen IP` \
        -e "s/-d ${docker_embedded_dns_ip}/-d ${docker_host_ip}/g" \
        `# we need to also apply these rules to non-local traffic (from pods)` \
        -e 's/-A OUTPUT \(.*\) -j DOCKER_OUTPUT/\0\n-A PREROUTING \1 -j DOCKER_OUTPUT/' \
        `# switch docker DNS SNAT rules rules to our chosen IP` \
        -e "s/--to-source :53/--to-source ${docker_host_ip}:53/g" \
        `# nftables incompatibility between 1.8.8 and 1.8.7 omit the --dport flag on DNAT rules` \
        `# ensure --dport on DNS rules, due to https://github.com/kubernetes-sigs/kind/issues/3054` \
        -e "s/p -j DNAT --to-destination ${docker_embedded_dns_ip}/p --dport 53 -j DNAT --to-destination ${docker_embedded_dns_ip}/g" \
      | "$iptables"-restore
  done

  # now we can ensure that DNS is configured to use our IP
  cp /etc/resolv.conf /etc/resolv.conf.original
  sed -e "s/${docker_embedded_dns_ip}/${docker_host_ip}/g" /etc/resolv.conf.original >/etc/resolv.conf

  # SPDX-SnippetEnd

  echo "$0: applied DNS fixes ($docker_embedded_dns_ip -> $docker_host_ip)" >&2
}

# Writes the k0s config from the environment variable to the config file.
write_k0s_config() {
  if [ -n "${K0S_CONFIG-}" ]; then
    mkdir -p /etc/k0s
    printf %s "$K0S_CONFIG" >/etc/k0s/config.yaml
  fi
}

# Determines the k0s role from the given command line.
k0s_role() {
  [ -z "${K0S_ENTRYPOINT_ROLE-}" ] || {
    echo "$K0S_ENTRYPOINT_ROLE"
    return
  }

  # scan cmdline if k0s is the executable
  [ "$(basename -- "${1-}")" = k0s ] || return
  shift

  while [ $# -gt 0 ]; do
    case "$1" in
    -*) shift ;;                     # skip all flags before first command
    worker) echo worker && return ;; # a worker is a worker
    controller)                      # examine controller flags
      shift
      while [ $# -gt 0 ]; do
        case "$1" in
        --single | --enable-worker) echo controller+worker && return ;;
        esac
        shift
      done

      echo controller
      return
      ;;
    *) return ;; # some other command
    esac
  done
}

main() {
  case "$(k0s_role "$@")" in
  worker | controller+worker)
    # Don't disable anything.
    ;;

  controller | *)
    # Disable everything that's only required when running nested containers.
    : "${K0S_ENTRYPOINT_REMOUNT_CGROUP2FS:=0}"
    : "${K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING:=0}"
    : "${K0S_ENTRYPOINT_DNS_FIXUP:=0}"
    ;;
  esac

  remount_cgroup2fs
  enable_cgroupv2_nesting
  dns_fixup
  write_k0s_config

  [ "${K0S_ENTRYPOINT_QUIET-}" = '1' ] || echo "$0: executing ${1-}" >&2

  exec env \
    -u K0S_ENTRYPOINT_QUIET \
    -u K0S_ENTRYPOINT_ROLE \
    -u K0S_ENTRYPOINT_REMOUNT_CGROUP2FS \
    -u K0S_ENTRYPOINT_ENABLE_CGROUPV2_NESTING \
    -u K0S_ENTRYPOINT_DNS_FIXUP \
    -u K0S_CONFIG \
    -- "$@"
}

case "$*" in
help | -h | --help) usage && exit 0 ;;
*) main "$@" ;;
esac
