resource "aws_instance" "cluster-workers" {
  count         = var.worker_count
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.instance_type
  tags = {
    Name = format("%s-worker-%d", local.cluster_unique_identifier, count.index)
    Role = "worker"
  }
  key_name                    = aws_key_pair.cluster-key.key_name
  subnet_id                   = aws_subnet.cluster-subnet.id
  vpc_security_group_ids      = [aws_security_group.cluster_allow_ssh.id]
  associate_public_ip_address = true
  source_dest_check           = false

  root_block_device {
    volume_type = "gp3"
    volume_size = 50
    iops        = 3000
  }

  connection {
    type        = "ssh"
    user        = "ubuntu"
    private_key = tls_private_key.k8s-conformance-key.private_key_pem
    host        = self.public_ip
    agent       = true
  }

  provisioner "remote-exec" {
    inline = [<<EOF
      sudo hostnamectl set-hostname worker-${count.index}
      sudo sh -c 'echo 127.0.0.1 worker-${count.index} >> /etc/hosts'
    EOF
    ]
  }
}
