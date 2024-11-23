
# sourceD

## Goal

`sourceD` is a Kubernetes operator developed by the [Versioneer](https://versioneer.at) team introducing a Kubernetes custom resource definition (CRD) called `Source` along with a controller, enabling Kubernetes users to manage S3-compatible storage services such as AWS S3, MinIO, or any other service supporting the S3 protocol. The `Source` CRD automates the creation and mounting of Persistent Volumes (PVs) backed by an S3 bucket using the [csi-rclone](https://github.com/wunderio/csi-rclone) CSI driver.

## How It Works

- **Persistent Volume and PVC Creation**: The `access` block in the `Source` spec allows the controller to automatically create and mount a Persistent Volume and Persistent Volume Claim (PVC) using the specified S3 bucket. The `csi-rclone` driver manages the connection and mounting of the storage.
- **Secret Injection for Temporary URL Generation**: The `share` block enables users to specify which credentials for the bucket should be injected into a target pod, allowing the pod to use those credentials to perform operations like generating temporary URLs for file sharing.
- **Credential Management**: S3 credentials are handled via Kubernetes Secrets. These secrets should contain the required AWS (or equivalent) credentials to access the S3 bucket and generate temporary URLs.

## Example `Source` CRD

```yaml
apiVersion: package.r/v1alpha1
kind: Source
metadata:
  name: example-source
spec:
  access:
    bucketName: "example-bucket"
    bucketPrefix: "test"
    secretName: "rw-credentials" # e.g. for read-write
  share:
    bucketName: "example-bucket"
    bucketPrefix: "test"
    secretName: "ro-credentials" # e.g. read-only
  friendlyName: "Example S3 Source"
```

## Example `Secret` for Credentials

For both `access` and `share` blocks, the `secretName` should refer to a Kubernetes secret containing the following keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: rw-credentials
data:
  AWS_ACCESS_KEY_ID: <base64-encoded-access-key>
  AWS_SECRET_ACCESS_KEY: <base64-encoded-secret-key>
  AWS_ENDPOINT_URL: <base64-encoded-endpoint-url>
  AWS_REGION: <base64-encoded-region>
```

## Getting Started

### Prerequisites
- Go
- Docker
- Access to a Kubernetes cluster via Kubecrl
- [`csi-rclone`](https://github.com/wunderio/csi-rclone) driver installed on your Kubernetes cluster  

### Usage

Once the `Source` CRD is applied, the controller will automatically create a Persistent Volume and Persistent Volume Claim backed by the specified S3 bucket. You can verify that the resources were created using:

```sh
kubectl get pv
kubectl get pvc
```

The PV should be configured with the `csi-rclone` driver and will be mounted to the Kubernetes cluster, providing access to the S3 bucket for your applications.

### Development & Deployment

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=ghcr.io/versioneer-tech/source-d:0.1
```

**Install/Uninstall the CRDs into the cluster:**

```sh
make install
make uninstall
```

**Deploy/Undeploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=ghcr.io/versioneer-tech/source-d:0.1
make undeploy
```

```sh
make build-installer IMG=ghcr.io/versioneer-tech/source-d:0.1
```

> **NOTE:** The `makefile` target mentioned above generates an `install.yaml` file in the `dist` directory. This file contains all the resources built with Kustomize, necessary for installing this project along with its dependencies.

Users can run `kubectl apply -f <URL for YAML BUNDLE>` to install the project, for example:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/source-d/<tag or branch>/dist/install.yaml
```

## License

[Apache 2.0](LICENSE) (Apache License Version 2.0, January 2004) from https://www.apache.org/licenses/LICENSE-2.0
