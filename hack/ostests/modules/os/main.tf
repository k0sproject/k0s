locals {
  # Boilerplate to make OpenTofu a little bit dynamic

  os = {
    al2023              = local.os_al2023
    alpine_3_22         = local.os_alpine_3_22
    alpine_3_24         = local.os_alpine_3_24
    centos_9            = local.os_centos_9
    centos_10           = local.os_centos_10
    debian_12           = local.os_debian_12
    debian_13           = local.os_debian_13
    fcos_stable         = local.os_fcos_stable
    fedora_45           = local.os_fedora_45
    flatcar             = local.os_flatcar
    rhel_8              = local.os_rhel_8
    rhel_9              = local.os_rhel_9
    rhel_10             = local.os_rhel_10
    rocky_8             = local.os_rocky_8
    rocky_9             = local.os_rocky_9
    rocky_10            = local.os_rocky_10
    sles_15             = local.os_sles_15
    ubuntu_2204         = local.os_ubuntu_2204
    ubuntu_2404         = local.os_ubuntu_2404
    ubuntu_2604         = local.os_ubuntu_2604
    windows_server_2022 = local.os_windows_server_2022
  }
}
