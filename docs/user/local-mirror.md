# Using a local mirror

Cluster installation depends on many images.
Users launching several clusters and users launching clusters detached from the broader internet can mirror the required images to a local registry for offline installation.

## Set up a local registry

In this section, I'll set up a local instance using Docker's registry image with a self-signed certificate.
See [here][docker-registry-deploy] for more details on deploying Docker's registry.
Feel free to substitute other registry implementations as you see fit.

### Address

We'll need this registry to be available from within the cluster (in this case, I'll launch the cluster on libvirt), so get our IP address:

```console
$ ip -4 addr show virbr0
4: virbr0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default qlen 1000
    inet 192.168.122.1/24 brd 192.168.122.255 scope global virbr0
       valid_lft forever preferred_lft forever
```

So my libvirt hosts will find my development host (and its local registry) at 192.168.122.1.

### X.509

Generate a self-signed certificate authority:

```console
$ mkdir -p certs
$ openssl req -newkey rsa:4096 -nodes -sha256 -keyout certs/registry.key -x509 -days 365 -subj '/CN=example.com/O=Example, Inc./C=US' -addext 'subjectAltName = IP:192.168.122.1' -out certs/registry.crt
```

`-addext` [requires OpenSSL 1.1.1+][openssl-addext].
For older version of OpenSSL, you can use:

```console
$ cat <<EOF >certs/registry.cnf
[ req ]
distinguished_name = req_dn
x509_extensions = req_ext

[ req_dn ]

[ req_ext ]
subjectAltName = IP:192.168.122.1
EOF
$ openssl req -config certs/registry.cnf -newkey rsa:4096 -nodes -sha256 -keyout certs/registry.key -x509 -days 365 -subj '/CN=example.com/O=Example, Inc./C=US' -out certs/registry.crt
```

### Launch

Launch the registry [using the self-signed certificate][docker-registry-tls]:

```sh
podman run -d -v "${PWD}/certs:/certs" -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt -e REGISTRY_HTTP_TLS_KEY=/certs/registry.key -p 5000:5000 --name registry docker.io/library/registry
```

### Firewall

You may need to allow TCP connections to the registry from the IP range used by your cluster nodes.
For more on this process, see [this documentation](../dev/libvirt-howto.md#firewall).

## Mirror images

Use [`oc`][oc] to mirror a particular OpenShift release to your local registry:

```console
$ oc adm release mirror ---insecure --from=registry.svc.ci.openshift.org/openshift/origin-release:v4.0 --to 192.168.122.1:5000/openshift-v4.0
info: Mirroring 79 images to 192.168.122.1:5000/openshift-v4.0 ...
...
info: Mirroring completed in 12m54.9s (7.057MB/s)

Success
Update image:  192.168.122.1:5000/openshift-v4.0:release
Mirror prefix: 192.168.122.1:5000/openshift-v4.0
```

If both your source and target registries support HTTPS with certificate authorities known to your local host, you can skip `--insecure`.
If you have not installed [your self-signed certificate](#x509), setting `--insecure` avoids `error: unable to connect to 192.168.122.1:5000/openshift-v4.0: Get https://192.168.122.1:5000/v2/: x509: certificate signed by unknown authority`.

`--insecure` is in flight with https://github.com/openshift/origin/pull/21266.

## Launch a cluster from the mirrored image

```sh
OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE=192.168.122.1:5000/openshift-v4.0:release _FIXME_OPENSHIFT_INSTALL_CERTIFICATE_AUTHORITY="$(cat certs/registry.crt)" openshift-install cluster
```

`_FIXME_OPENSHIFT_INSTALL_CERTIFICATE_AUTHORITY` is in flight with https://github.com/openshift/installer/pull/472.

Getting much savings may also be blocked on https://github.com/openshift/cluster-version-operator/pull/39, since the mirror is currently an identical copy:

```console
$ podman images --digests | head -n3
REPOSITORY                                               TAG           DIGEST                                                                    IMAGE ID       CREATED        SIZE
registry.svc.ci.openshift.org/openshift/origin-release   v4.0          sha256:b2f169a1ec505edaec3d39f05632568082a18be3548b475c42285c8ec48c30a5   613f292ebcd1   5 hours ago    285MB
192.168.122.1:5000/openshift-v4.0                        release       sha256:b2f169a1ec505edaec3d39f05632568082a18be3548b475c42285c8ec48c30a5   613f292ebcd1   5 hours ago    285MB
```

so without rewriting logic we'd pull all the images after the first from their original registry (we want to pull them from our local registry too).

[docker-registry-deploy]: https://docker.github.io/registry/deploying/
[docker-registry-tls]: https://docker.github.io/registry/deploying/#get-a-certificate
[oc]: https://github.com/openshift/origin/tree/master/cmd/oc
[openssl-addext]: https://github.com/openssl/openssl/commit/bfa470a4f64313651a35571883e235d3335054eb
