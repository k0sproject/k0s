/*
data "aws_availability_zones" "available" {
  state = "available"
}
*/

resource "aws_instance" "cluster-workers" {
  count         = var.worker_count
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.cluster_flavor
  tags = {
    Name = format("%s-worker-%d", var.cluster_name, count.index)
  }
  key_name                    = aws_key_pair.cluster-key.key_name
  subnet_id                   = aws_subnet.cluster-subnet.id
  vpc_security_group_ids      = [aws_security_group.cluster_allow_ssh.id]
  associate_public_ip_address = true
  // availability_zone           = data.aws_availability_zones.available.names[count.index]

  root_block_device {
    volume_type = "gp2"
    volume_size = 20
  }

  connection {
    type        = "ssh"
    user        = "ubuntu"
    private_key = tls_private_key.k8s-conformance-key.private_key_pem
    host        = self.public_ip
    agent       = true
  }

  provisioner "remote-exec" {
    inline = [
      "sudo hostnamectl set-hostname worker-${count.index}",
      "sudo add-apt-repository -y ppa:longsleep/golang-backports && sudo apt update",
      "sudo apt install -y golang-go",
      "sudo apt install -y make",
      "curl -fsSL https://get.docker.com -o get-docker.sh",
      "sudo sh get-docker.sh",
      "sudo usermod -aG docker $USER"
    ]
  }
}


output "worker-external-ip" {
  value = aws_instance.cluster-workers.*.public_ip
}
