# k0s Single Node Quick Start
These instructions outline a quick method for running a local k0s master and worker in a single node.

> **_NOTE:_**  This method of running k0s is only recommended for dev, test or POC environments.

## Prepare Dependencies
#### 1. Download the k0s binary
```sh
curl -sSLf https://get.k0s.sh | sh
```

#### 2. Download the kubectl binary
```sh
sudo curl --output /usr/local/sbin/kubectl -L "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
```

#### 3. Make both binaries executable
```sh
sudo chmod +x /usr/local/sbin/kubectl
sudo chmod +x /usr/bin/k0s
```

## Start k0s
#### 1. Create the k0s config directory
```sh
mkdir -p ${HOME}/.k0s
```

#### 2. Generate a default cluster configuration
```sh
k0s default-config | tee ${HOME}/.k0s/k0s.yaml
```

#### 3. Start k0s
```sh
sudo k0s server -c ${HOME}/.k0s/k0s.yaml --enable-worker &
```

## Use kubectl to access k0s
#### 1. Save kubeconfig for user
```sh
sudo cat /var/lib/k0s/pki/admin.conf | tee ~/.k0s/kubeconfig
```

#### 2. Set the KUBECONFIG environment variable
```sh
export KUBECONFIG="${HOME}/.k0s/kubeconfig"
```

#### 3. Monitor cluster startup
```sh
kubectl get pods --all-namespaces
```
