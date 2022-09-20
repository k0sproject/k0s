output "k0s_update_binary_url" {
  value = data.local_file.k0s_update_binary_presigned_url.content
}

output "k0s_update_airgap_bundle_url" {
  value = var.k0s_airgap_bundle != "" ? data.local_file.k0s_airgap_bundle_presigned_url[0].content : ""
}
