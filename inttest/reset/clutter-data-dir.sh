# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2024 k0s authors
#shellcheck shell=ash

set -eu

make_dir() { mkdir -- "$1" && echo "$1"; }
make_file() { echo "$1" >"$1" && echo "$1"; }

make_bind_mounts() {
  local real="$1"
  local target="$2"

  # Directory bind mount
  make_dir "$real/real_dir"
  make_file "$real/real_dir/real_dir_info.txt"
  make_dir "$target/bind_dir"
  mount --bind -- "$real/real_dir" "$target/bind_dir"

  # File bind mount
  make_file "$real/real_file.txt"
  make_file "$target/bind_file.txt"
  mount --bind -- "$real/real_file.txt" "$target/bind_file.txt"

  # Recursive directory bind mount
  make_dir "$real/real_recursive_dir"
  make_file "$real/real_recursive_dir/real_recursive_dir.txt"
  make_dir "$real/real_recursive_dir/bind_dir"
  mount --bind -- "$real/real_dir" "$real/real_recursive_dir/bind_dir"
  make_file "$real/real_recursive_dir/bind_file.txt"
  mount --bind -- "$real/real_file.txt" "$real/real_recursive_dir/bind_file.txt"
  make_dir "$target/rbind_dir"
  mount --rbind -- "$real/real_recursive_dir" "$target/rbind_dir"

  # Directory overmounts
  make_dir "$real/overmount_dir"
  make_file "$real/overmount_dir/in_overmount_dir.txt"
  mount --bind -- "$real/overmount_dir" "$target/bind_dir"

  # File overmounts
  make_file "$real/overmount_file.txt"
  mount --bind -- "$real/overmount_file.txt" "$target/bind_file.txt"
}

clutter() {
  local dataDir="$1"
  local realDir

  realDir="$(mktemp -t -d k0s_reset_inttest.XXXXXX)"

  local dir="$dataDir"/cluttered
  make_dir "$dir"

  # Directories and files with restricted permissions
  make_dir "$dir/restricted_dir"
  make_file "$dir/restricted_dir/no_read_file.txt"
  chmod 000 -- "$dir/restricted_dir/no_read_file.txt" # No permissions on the file
  make_dir "$dir/restricted_dir/no_exec_dir"
  chmod 000 -- "$dir/restricted_dir/no_exec_dir" # No permissions on the directory
  make_dir "$dir/restricted_dir/no_exec_nonempty_dir"
  make_file "$dir/restricted_dir/no_exec_nonempty_dir/.hidden_file"
  chmod 000 -- "$dir/restricted_dir/no_exec_nonempty_dir" # No permissions on the directory

  # Symlinks pointing outside the directory tree
  make_dir "$realDir/some_dir"
  make_file "$realDir/some_dir/real_file.txt"
  ln -s -- "$realDir/some_dir/real_file.txt" "$dir/symlink_to_file" # Symlink to a file
  ln -s -- "$realDir/some_dir" "$dir/symlink_to_dir"                # Symlink to a directory

  # Bind mounts pointing outside the directory tree
  make_bind_mounts "$realDir" "$dir"

  # Bind mounts outside the directory tree pointing into it
  # make_bind_mounts "$dir" "$realDir"
}

clutter "$@"
