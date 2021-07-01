# Uninstall/Reset

k0s can be uninstalled locally with `k0s reset` command and remotely with `k0sctl reset` command. They remove all k0s-related files from the host.

`reset` operates under the assumption that k0s is installed as a service on the host.

## Uninstall a k0s node locally

To prevent accidental triggering, `k0s reset` will not run if the k0s service is running, so you must first stop the service:

1. Stop the service:

    ```shell
    sudo k0s stop
    ```

2. Invoke the `reset` command:

    ```shell
    $ sudo k0s reset
    INFO[2021-06-29 13:08:39] * containers steps
    INFO[2021-06-29 13:08:44] successfully removed k0s containers!
    INFO[2021-06-29 13:08:44] no config file given, using defaults
    INFO[2021-06-29 13:08:44] * remove k0s users step:
    INFO[2021-06-29 13:08:44] no config file given, using defaults
    INFO[2021-06-29 13:08:44] * uninstal service step
    INFO[2021-06-29 13:08:44] Uninstalling the k0s service
    INFO[2021-06-29 13:08:45] * remove directories step
    INFO[2021-06-29 13:08:45] * CNI leftovers cleanup step
    INFO k0s cleanup operations done. To ensure a full reset, a node reboot is recommended.
    ```

## Uninstall a k0s cluster using k0sctl

k0sctl can be used to connect each node and remove all k0s-related files and processes from the hosts.

1. Invoke `k0sctl reset` command:

    ```shell
    $ k0sctl reset --config k0sctl.yaml
    k0sctl v0.9.0 Copyright 2021, k0sctl authors.

    ? Going to reset all of the hosts, which will destroy all configuration and data, Are you sure? Yes
    INFO ==> Running phase: Connect to hosts 
    INFO [ssh] 13.53.43.63:22: connected              
    INFO [ssh] 13.53.218.149:22: connected            
    INFO ==> Running phase: Detect host operating systems 
    INFO [ssh] 13.53.43.63:22: is running Ubuntu 20.04.2 LTS 
    INFO [ssh] 13.53.218.149:22: is running Ubuntu 20.04.2 LTS 
    INFO ==> Running phase: Prepare hosts    
    INFO ==> Running phase: Gather k0s facts 
    INFO [ssh] 13.53.43.63:22: found existing configuration 
    INFO [ssh] 13.53.43.63:22: is running k0s controller version 1.21.2+k0s.0 
    INFO [ssh] 13.53.218.149:22: is running k0s worker version 1.21.2+k0s.0 
    INFO [ssh] 13.53.43.63:22: checking if worker  has joined 
    INFO ==> Running phase: Reset hosts      
    INFO [ssh] 13.53.43.63:22: stopping k0s           
    INFO [ssh] 13.53.218.149:22: stopping k0s         
    INFO [ssh] 13.53.218.149:22: running k0s reset    
    INFO [ssh] 13.53.43.63:22: running k0s reset      
    INFO ==> Running phase: Disconnect from hosts 
    INFO ==> Finished in 8s                  
    ```