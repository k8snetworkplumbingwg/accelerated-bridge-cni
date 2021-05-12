[![Build Status](https://travis-ci.org/mellanox/accelerated-bridge-cni.svg?branch=master)](https://travis-ci.org/mellanox/accelerated-bridge-cni) [![Go Report Card](https://goreportcard.com/badge/github.com/mellanox/accelerated-bridge-cni)](https://goreportcard.com/report/github.com/mellanox/accelerated-bridge-cni)

   * [Accelerated Bridge CNI plugin](#accelerated-bridge-cni-plugin)
      * [Build](#build)
      * [Kubernetes Quick Start](#kubernetes-quick-start)
      * [Usage](#usage)
         * [Basic configuration parameters](#basic-configuration-parameters)
         * [Example configurations](#example-configurations)
            * [Kernel driver config](#kernel-driver-config)
            * [Advanced kernel driver config](#advanced-kernel-driver-config)
            * [DPDK userspace driver config](#dpdk-userspace-driver-config)
         * [Advanced configuration](#advanced-configuration)
      * [Contributing](#contributing)

# Accelerated Bridge CNI plugin
This plugin enables the configuration and usage of Accelerated Bridge VF networks in containers and orchestrators like Kubernetes.

Network Interface Cards (NICs) with [SR-IOV](http://blog.scottlowe.org/2009/12/02/what-is-sr-iov/) capabilities are managed through physical functions (PFs) and virtual functions (VFs). A PF is used by the host and usually represents a single NIC port. VF configurations are applied through the PF. With Accelerated Bridge CNI each VF can be treated as a separate network interface, assigned to a container, and configured with it's own MAC, VLAN IP and more.

Accelerated Bridge CNI plugin works with [SR-IOV device plugin](https://github.com/intel/sriov-network-device-plugin) for VF allocation in Kubernetes. A metaplugin such as [Multus](https://github.com/intel/multus-cni) gets the allocated VF's `deviceID`(PCI address) and is responsible for invoking the Accelerated Bridge CNI plugin with that `deviceID`.

## Build

This plugin uses Go modules for dependency management and requires Go 1.13+ to build.

To build the plugin binary:

``
make
``

Upon successful build the plugin binary will be available in `build/accelerated-bridge`.

## Kubernetes Quick Start
A full guide on orchestrating Accelerated Bridge virtual functions in Kubernetes can be found at the [Accelerated Bridge Device Plugin project.](https://github.com/intel/sriov-network-device-plugin#quick-start)

Creating VFs is outside the scope of the Accelerated Bridge CNI plugin. [More information about allocating VFs on different NICs can be found here](https://github.com/intel/sriov-network-device-plugin/blob/master/docs/vf-setup.md)

To deploy Accelerated Bridge CNI by itself on a Kubernetes 1.16+ cluster:

`kubectl apply -f images/k8s-v1.16/accelerated-bridge-cni-daemonset.yaml`

**Note** The above deployment is not sufficient to manage and configure Accelerated Bridge virtual functions. [See the full orchestration guide for more information.](https://github.com/intel/sriov-network-device-plugin#sr-iov-network-device-plugin)


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

#### Kernel driver config
This is the minimum configuration for a working kernel driver interface using an Accelerated Bridge Virtual Function. It applies an IP address using the host-local IPAM plugin in the range of the subnet provided.

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

#### Extended kernel driver config
This configuration sets a number of extra parameters that may be key for Accelerated Bridge networks including a vlan tag, disabled spoof checking and enabled trust mode. These parameters are commonly set in more advanced Accelerated Bridge VF based networks.

```json
{
  "cniVersion": "0.3.1",
  "name": "some-net-advanced",
  "type": "accelerated-bridge",
  "vlan": 1000,
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
