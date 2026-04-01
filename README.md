![hcloud-cosi-driver](docs/picture.png)


# hcloud-cosi-driver

A Kubernetes [COSI](https://github.com/kubernetes-sigs/container-object-storage-interface) driver for [Hetzner Object Storage](https://www.hetzner.com/storage/object-storage).

Buckets are provisioned via the Hetzner Cloud API. S3 credentials are bootstrapped once on startup and shared across all bucket access grants.

## Prerequisites

- Kubernetes cluster with the [COSI controller](https://github.com/kubernetes-sigs/container-object-storage-interface) installed
- A `hcloud` Secret in `kube-system` with your Hetzner API token (same one used by [hcloud-csi-driver](https://github.com/hetznercloud/csi-driver)):

```bash
kubectl create secret generic hcloud \
  --namespace kube-system \
  --from-literal=token=<your-hcloud-api-token>
```

## Install

```bash
helm install hcloud-cosi-driver oci://ghcr.io/espresso-lab/helm-charts/hcloud-cosi-driver \
  --namespace kube-system \
  --version <version>
```

## Create a bucket in fsn1

**1. Create a BucketClass referencing the driver and location:**

```yaml
apiVersion: objectstorage.k8s.io/v1alpha1
kind: BucketClass
metadata:
  name: hcloud-fsn1
driverName: hcloud.espressolab.objectstorage.k8s.io
deletionPolicy: Delete
parameters:
  location: fsn1
```

**2. Create a BucketClaim:**

```yaml
apiVersion: objectstorage.k8s.io/v1alpha1
kind: BucketClaim
metadata:
  name: my-bucket
  namespace: default
spec:
  bucketClassName: hcloud-fsn1
  protocols:
    - S3
```

**3. Create a BucketAccessClass and BucketAccess to get credentials into your pod:**

```yaml
apiVersion: objectstorage.k8s.io/v1alpha1
kind: BucketAccessClass
metadata:
  name: hcloud-fsn1
driverName: hcloud.espressolab.objectstorage.k8s.io
authenticationType: Key
---
apiVersion: objectstorage.k8s.io/v1alpha1
kind: BucketAccess
metadata:
  name: my-bucket-access
  namespace: default
spec:
  bucketClaimName: my-bucket
  bucketAccessClassName: hcloud-fsn1
  credentialsSecretName: my-bucket-credentials
  protocol: S3
```

Once reconciled, `my-bucket-credentials` will contain the S3 `accessKeyID`, `accessSecretKey`, `endpoint`, and `bucketName` for `https://fsn1.your-objectstorage.com`.

