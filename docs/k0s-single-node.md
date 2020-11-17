# k0s Single Node Quick Start
This outlines a quick method to running local k0s master and worker in one node. 

  1) Prepare Dependencies
      1. Download the k0s binary
      ```sh
       sudo curl --output /usr/local/sbin/k0s -L https://github.com/k0sproject/k0s/releases/download/v0.7.0/k0s-v0.7.0-amd64
      ```
      2. Download the kubectl binary
      ```sh
       sudo curl --output /usr/local/sbin/kubectl -L "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
      ```
      3. Make both binaries executeable
      ```sh
       sudo chmod +x /usr/local/sbin/{kubectl,k0s}
      ```

  2) Start k0s
      1. Create ~/.k0s directory
      ```sh
       mkdir -p ${HOME}/.k0s
      ```
      2. Generate default k0s.yaml cluster configuration
      ```sh
       k0s default-config | tee ${HOME}/.k0s/k0s.yaml
      ```
      3. Start k0s
      ```sh
       sudo k0s server -c ${HOME}/k0s.yaml --enable-worker &
      ```

  3) Monitor Startup k0s
      1. Save kubeconfig for user
      ```sh
       sudo cat /var/lib/k0s/pki/admin.conf | tee ~/.k0s/kubeconfig
      ```
      2. Set the KUBECONFIG environment variable
      ```sh
       export KUBECONFIG="${HOME}/.k0s/kubeconfig"
      ```
      3. Monitor cluster startup
      ```sh
       kubectl get pods --all-namespaces
      ```