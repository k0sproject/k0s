# Providers

We use Google Cloud as a provider for the sake of the example. Check your vendor documentation in case if you use some other vendor.

## Notes on stand-alone providers

If you are using stand-alone OIDC provider, you might need to specify `oidc-ca-file` argument for the kube-api.

## Google Cloud

We use [k8s-oidc-helper](https://github.com/micahhausler/k8s-oidc-helper) tool to create proper kubeconfig user record.

The issuer URL for the Google cloud is `https://accounts.google.com`

### Creating an application

- Go to the [Google Cloud Dashboard](https://console.developers.google.com/apis/dashboard)
- Create a new project in your organization
- Go to the "Credentials" page
- Create "OAuth consent screen"

### Creating a user credentials

- Go to the [Google Cloud Dashboard](https://console.developers.google.com/apis/dashboard)
- Go to the "Credentials" page
- Create new credentials. Select "OAuth client ID" as a type.
- Select "Desktop" app as an application type.
- Save client ID and client secret

### Creating kubeconfig user record

Use the command and follow the instructions:

```bash
k8s-oidc-helper --client-id=<CLIENT_ID> \
  --client-secret=<CLIENT_SECRET> \
  --write=true
```

## Using kubelogin

For other OIDC providers it is possible to use `kubelogin` plugin.
Please refer to the [setup guide](https://github.com/int128/kubelogin/blob/master/docs/setup.md) for details.

### Google Cloud example using `kubelogin`

```bash
kubectl oidc-login setup \
  --oidc-issuer-url=https://accounts.google.com \
  --oidc-client-id=<CLIENT_ID> \
  --oidc-client-secret=<CLIENT_SECRET>

  kubectl config set-credentials oidc \
  --exec-api-version=client.authentication.k8s.io/v1beta1 \
  --exec-command=kubectl \
  --exec-arg=oidc-login \
  --exec-arg=get-token \
  --exec-arg=--oidc-issuer-url=https://accounts.google.com \
  --exec-arg=--oidc-client-id=<CLIENT_ID>  \
  --exec-arg=--oidc-client-secret=<CLIENT_SECRET>
```

You can switch the current context to oidc.

```kubectl config set-context --current --user=oidc```
