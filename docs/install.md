# Service and Logging 

We've created the subcommand `k0s install` to allow users to easily install k0s as a service, and define its logging.

This is an **alpha state** feature.

## Caveats
* This command is strictly a helper command. It is not meant to provide a fully-automated solution, since you can run k0s in multiple, very different ways.
* It configures your service set-up as either a worker or a server, and will have different tasks, depending on the role you pick.
* Supported services: OpenRC & Systemd

### Server setup
This is the default mode of operation. When a server role is picked, the installer will do the following:
* Create user accounts for the different components (see https://github.com/k0sproject/k0s/blob/main/pkg/apis/v1beta1/system.go#L6)
* Create a service file (OpenRC/Systemd) and redirects logging to `/var/log/k0s.log`.
* If the `--debug` flag is used, it will also pass this flag along to the service file.
* `enable-worker` (single-node) setup is not supported. If you would like to run your service in that way, a possible solution would be to run `cmd install ` as worker, and edit the startup command by hand.

### Worker Setup
* A worker cannot run with any other user, other than `root`, so no special users will be created.
* The service file will include the `--token-file` flag, with a value that needs to be manually changed.
* If the `--debug` flag is used, it will also pass this flag along to the service file.

### Additional Documentation
see: [k0s install](cli/k0s_install.md)