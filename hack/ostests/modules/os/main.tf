locals {
  # Boilerplate to make OpenTofu a little bit dynamic

  os = {
    al2023              = local.os_al2023
    alpine_3_17         = local.os_alpine_3_17
    alpine_3_20         = local.os_alpine_3_20
    centos_7            = local.os_centos_7
    centos_8            = local.os_centos_8
    centos_9            = local.os_centos_9
    debian_10           = local.os_debian_10
    debian_11           = local.os_debian_11
    debian_12           = local.os_debian_12
    fcos_38             = local.os_fcos_38
    fedora_38           = local.os_fedora_38
    flatcar             = local.os_flatcar
    oracle_7_9          = local.os_oracle_7_9
    oracle_8_7          = local.os_oracle_8_7
    oracle_9_1          = local.os_oracle_9_1
    rhel_7              = local.os_rhel_7
    rhel_8              = local.os_rhel_8
    rhel_9              = local.os_rhel_9
    rocky_8             = local.os_rocky_8
    rocky_9             = local.os_rocky_9
    ubuntu_2004         = local.os_ubuntu_2004
    ubuntu_2204         = local.os_ubuntu_2204
    ubuntu_2304         = local.os_ubuntu_2304
    windows_server_2022 = local.os_windows_server_2022
  }
}
