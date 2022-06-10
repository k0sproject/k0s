variable "k0s_version" {
  type    = string
}

variable "sonobuoy_version" {
  type    = string
  default = "0.53.2"
}

resource "random_id" "cluster_identifier" {
  byte_length = 4
}

module "k0s-sonobuoy" {
  source = "../../terraform/test-cluster"
  cluster_name = "sonobuoy_test-${random_id.cluster_identifier.hex}"
  cluster_flavor = "c4.xlarge"
}

resource "null_resource" "controller" {
  depends_on = [module.k0s-sonobuoy]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.controller_external_ip[0]
    agent       = true
    user        = "ubuntu"
  }
}

resource "null_resource" "configure_worker1" {
  depends_on = [null_resource.controller]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.worker_external_ip[0]
    agent       = true
    user        = "ubuntu"
  }


  provisioner "file" {
    source      = module.k0s-sonobuoy.controller_pem.filename
    destination = "/home/ubuntu/.ssh/id_rsa"
  }
}

resource "null_resource" "configure_worker2" {
  depends_on = [null_resource.controller]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.worker_external_ip[1]
    agent       = true
    user        = "ubuntu"
  }

  provisioner "file" {
    source      = module.k0s-sonobuoy.controller_pem.filename
    destination = "/home/ubuntu/.ssh/id_rsa"
  }
}


resource "null_resource" "sonobuoy" {
  depends_on = [null_resource.configure_worker2]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.controller_external_ip[0]
    agent       = true
    user        = "ubuntu"
  }

  provisioner "remote-exec" {
    inline = [
      "wget https://github.com/vmware-tanzu/sonobuoy/releases/download/v${var.sonobuoy_version}/sonobuoy_${var.sonobuoy_version}_linux_amd64.tar.gz",
      "tar -xvf sonobuoy_${var.sonobuoy_version}_linux_amd64.tar.gz",
      "sudo mv sonobuoy /usr/local/bin",
      "sudo chmod +x /usr/local/bin/sonobuoy",
    ]
  }
}
