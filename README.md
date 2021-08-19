## About
This repository implements a simple controller with Kubebuilder for watching Foo resources.
### Kubebuilder
Kubebuilder is a framework for building Kubernetes APIs using custom resource definitions (CRDs).
#### Create a project
> Make sure you’ve installed Kubebuilder.
```
kubebuilder init --domain hmsayem.kubebuilder.io
```
#### Create an API
```
kubebuilder create api --group batch --version v1 --kind Foo
```

### How to run

Copy all third-party dependencies to a vendor folder in your project root.
```
go mod vendor
```

Install the CRD into the K8s cluster specified in ~/.kube/config.
```
make install
```
Install an instance of the Custom Resource.
```
kubectl apply -f config/samples/foo.yml
```

Run the controller from the host.
```
make run
```
#### Reference
[KubeBuilder](https://book.kubebuilder.io)

