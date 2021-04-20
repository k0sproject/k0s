module "k0s-sonobuoy" {
  source       = "github.com/k0sproject/k0s/inttest/terraform/test-cluster"
  cluster_name = "sonobuoy_test"
}

variable "k0s_version" {
  type    = string
}

variable "sonobuoy_version" {
  type    = string
  default = "0.20.0"
}

variable "k8s_version" {
  // format: v1.20.6
  type = string
}

output "controller_ip" {
  value = module.k0s-sonobuoy.controller_external_ip
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


  provisioner "remote-exec" {
    inline = [
      "sudo curl -SsLf get.k0s.sh | sudo K0S_VERSION=${var.k0s_version} sh",
      "sudo nohup k0s controller --enable-worker >/home/ubuntu/k0s-master.log 2>&1 &",
      "echo 'Wait 10 seconds for cluster to start!!!'",
      "sleep 10",
      "sudo snap install kubectl --classic"
    ]
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

  provisioner "file" {
    source      = "./startworker.sh"
    destination = "/home/ubuntu/startworker.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo curl -SsLf get.k0s.sh | sudo K0S_VERSION=${var.k0s_version} sh",
      "sudo chmod +x /home/ubuntu/startworker.sh ",
      "sudo /home/ubuntu/startworker.sh ${module.k0s-sonobuoy.controller_external_ip[0]}",
      "echo 'Wait 10 seconds for worker to start!!!'",
      "sleep 10",
    ]
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

  provisioner "file" {
    source      = "./startworker.sh"
    destination = "/home/ubuntu/startworker.sh"
  }


  provisioner "remote-exec" {
    inline = [
      "sudo curl -SsLf get.k0s.sh | sudo K0S_VERSION=${var.k0s_version} sh",
      "sudo chmod +x /home/ubuntu/startworker.sh",
      "sudo /home/ubuntu/startworker.sh  ${module.k0s-sonobuoy.controller_external_ip[0]}",
      "echo 'Wait 10 seconds for worker to start!!!'",
      "sleep 10",
    ]
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
      "sudo KUBECONFIG=/var/lib/k0s/pki/admin.conf sonobuoy run --mode=certified-conformance --kube-conformance-image-version=${var.k8s_version}"
    ]
  }
}
