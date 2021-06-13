[![Build](https://github.com/k8snetworkplumbingwg/accelerated-bridge-cni/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/k8snetworkplumbingwg/accelerated-bridge-cni/actions/workflows/build.yml)
[![Publish image](https://github.com/k8snetworkplumbingwg/accelerated-bridge-cni/actions/workflows/image-push.yml/badge.svg?branch=master)](https://github.com/k8snetworkplumbingwg/accelerated-bridge-cni/actions/workflows/image-push.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/k8snetworkplumbingwg/accelerated-bridge-cni)](https://goreportcard.com/report/github.com/k8snetworkplumbingwg/accelerated-bridge-cni)
[![Coverage Status](https://coveralls.io/repos/github/k8snetworkplumbingwg/accelerated-bridge-cni/badge.svg?branch=master)](https://coveralls.io/github/k8snetworkplumbingwg/accelerated-bridge-cni?branch=master)
   * [Accelerated Bridge CNI plugin](#accelerated-bridge-cni-plugin)
      * [Build](#build)
      * [Kubernetes Quick Start](#kubernetes-quick-start)
      * [Usage](#usage)
         * [Basic configuration parameters](#basic-configuration-parameters)
         * [Example configurations](#example-configurations)
            * [Basic config](#basic-config)
            * [Extended config](#extended-config)
         * [Advanced configuration](#advanced-configuration)
      * [Contributing](#contributing)

# Accelerated Bridge CNI plugin
This plugin allows using Linux Bridge with hardware offloading in containers and orchestrators like Kubernetes.
Accelerated Bridge CNI plugin requires NIC with support of SR-IOV
technology with VFs in switchdev mode and support of Linux Bridge offloading.

In switchdev mode, two net-devs exist for VF net-device and VF representor.
A packet sent through the VF representor on the host arrives to the VF, and a packet sent through the VF is received by its representor.
CNI plugin moves VF net-device to a container network namespace and attaches VF representor to a Linux Bridge on a host.
Then additional Bridge port settings, such as VLANs configuration, can be applied for VF representor.

This plugin works with [SR-IOV device plugin](https://github.com/intel/sriov-network-device-plugin) for VF allocation in Kubernetes.

A metaplugin such as [Multus](https://github.com/intel/multus-cni) gets the allocated VF's `deviceID`(PCI address) and is responsible for invoking the Accelerated Bridge CNI plugin with that `deviceID`.

Accelerated Bridge plugin assumes that Linux Bridge is already exist and correctly configured on nodes.

## Build

This plugin uses Go modules for dependency management and requires Go 1.13+ to build.

To build the plugin binary:

``
make
``

Upon successful build the plugin binary will be available in `build/accelerated-bridge`.

## Kubernetes Quick Start
A full guide on orchestrating SR-IOV virtual functions in Kubernetes can be found at the [SR-IOV Network Device Plugin project.](https://github.com/intel/sriov-network-device-plugin#quick-start)

Creating VFs is outside the scope of the Accelerated Bridge CNI plugin. [More information about allocating VFs on different NICs can be found here](https://github.com/intel/sriov-network-device-plugin/blob/master/docs/vf-setup.md)

To deploy Accelerated Bridge CNI by itself on a Kubernetes 1.16+ cluster:

`kubectl apply -f images/k8s-v1.16/accelerated-bridge-cni-daemonset.yaml`


## Usage
Accelerated Bridge CNI networks are commonly configured using Multus and SR-IOV Device Plugin using Network Attachment Definitions. More information about configuring Kubernetes networks using this pattern can be found in the [Multus configuration reference document.](https://intel.github.io/multus-cni/doc/configuration.html)

A Network Attachment Definition for Accelerated Bridge CNI takes the form:

```
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: some-net
  annotations:
    k8s.v1.cni.cncf.io/resourceName: intel.com/intel_sriov_netdevice
spec:
  config: '{
  "type": "accelerated-bridge",
  "cniVersion": "0.3.1",
  "name": "some-net",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}'
```

The `.spec.config` field contains the configuration information used by the Accelerated Bridge CNI.

### Basic configuration parameters

The following parameters are generic parameters which are not specific to the Accelerated Bridge CNI configuration, though (with the exception of ipam) they need to be included in the config.

* `cniVersion` : the version of the CNI spec used.
* `type` : CNI plugin used. "accelerated-bridge" corresponds to Accelerated Bridge CNI.
* `name` : the name of the network created.
* `ipam` (optional) : the configuration of the IP Address Management plugin. Required to designate an IP for a kernel interface.

### Example configurations
The following examples show the config needed to set up basic Accelerated Bridge networking in a container. Each of the json config objects below can be placed in the `.spec.config` field of a Network Attachment Definition to integrate with Multus.

#### Basic config
This is the minimum configuration for Accelerated Bridge CNI. It applies an IP address using the host-local IPAM plugin in the range of the subnet provided.

```json
{
  "type": "accelerated-bridge",
  "cniVersion": "0.3.1",
  "name": "some-net",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}
```
_Note: by default Accelerated Bridge CNI will use Linux Bridge with name `cni0`_

#### Extended config
This configuration sets a number of extra parameters that may be key for Accelerated Bridge networks including a vlan tag, disabled spoof checking and enabled trust mode. These parameters are commonly set in more advanced Accelerated Bridge VF based networks.

```json
{
  "cniVersion": "0.3.1",
  "name": "some-net-advanced",
  "type": "accelerated-bridge",
  "vlan": 1000,
  "bridge": "br1",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}
```


### Advanced Configuration

To learn more about available configuration parameters, check [Accelerated Bridge CNI configuration reference guide](docs/configuration-reference.md)

## Contributing
To report a bug or request a feature, open an issue on this repo using one of the available templates.
