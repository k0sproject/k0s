# Uninstall the k0s Cluster

Use the k0s CLI `reset` command to uninstall k0s, by removing all k0s-related files from the host.

`reset` operates under the assumption that k0s is installed as a service on the host. To prevent accidental triggering, the command will not run if the k0s service is running, so you must first stop the service: 

1. Stop the controller nodes:

    ```sh
    $ systemctl stop k0scontroller
    ```

2. Invoke the `reset` command:
 
    ```sh
    $ k0s reset
    INFO[2021-02-25 15:58:41] Uninstalling the k0s service                 
    INFO[2021-02-25 15:58:42] no config file given, using defaults         
    INFO[2021-02-25 15:58:42] deleting user: etcd                          
    INFO[2021-02-25 15:58:42] deleting user: kube-apiserver                
    INFO[2021-02-25 15:58:42] deleting user: konnectivity-server           
    INFO[2021-02-25 15:58:42] deleting user: kube-scheduler                
    INFO[2021-02-25 15:58:42] starting containerd for cleanup operations... 
    INFO[2021-02-25 15:58:42] containerd succesfully started               
    INFO[2021-02-25 15:58:42] attempting to clean up kubelet volumes...    
    INFO[2021-02-25 15:58:42] successfully removed kubelet mounts!         
    INFO[2021-02-25 15:58:42] attempting to clean up network namespaces... 
    INFO[2021-02-25 15:58:42] successfully removed network namespaces!     
    INFO[2021-02-25 15:58:42] attempting to stop containers...             
    INFO[2021-02-25 15:58:49] successfully removed k0s containers!         
    INFO[2021-02-25 15:58:49] deleting k0s generated data-dir (/var/lib/k0s) and run-dir (/run/k0s) 
    ERRO[2021-02-25 15:58:50] k0s cleanup operations done. To ensure a full reset, a node reboot is recommended. 
    ```
