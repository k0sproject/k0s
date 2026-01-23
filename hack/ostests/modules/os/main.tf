locals {
  # Boilerplate to make OpenTofu a little bit dynamic

  os = {
    al2023              = local.os_al2023
    alpine_3_19         = local.os_alpine_3_19
    alpine_3_23         = local.os_alpine_3_23
    centos_9            = local.os_centos_9
    centos_10           = local.os_centos_10
    debian_11           = local.os_debian_11
    debian_12           = local.os_debian_12
    fcos_stable         = local.os_fcos_stable
    fedora_41           = local.os_fedora_41
    flatcar             = local.os_flatcar
    oracle_8_9          = local.os_oracle_8_9
    oracle_9_3          = local.os_oracle_9_3
    rhel_7              = local.os_rhel_7
    rhel_8              = local.os_rhel_8
    rhel_9              = local.os_rhel_9
    rocky_8             = local.os_rocky_8
    rocky_9             = local.os_rocky_9
    sles_15             = local.os_sles_15
    ubuntu_2004         = local.os_ubuntu_2004
    ubuntu_2204         = local.os_ubuntu_2204
    ubuntu_2404         = local.os_ubuntu_2404
    windows_server_2022 = local.os_windows_server_2022
  }
}
