## Configuration reference - Accelerated Bridge CNI

The Accelerated Bridge CNI configures networks through a CNI spec configuration object. In a Kubernetes cluster set up with Multus this object is most often delivered as a Network Attachment Definition.


### Parameters
* `name` (string, required): the name of the network
* `type` (string, required): "accelerated-bridge"
* `ipam` (dictionary, optional): IPAM configuration to be used for this network.
* `deviceID` (string, required): A valid pci address of a SWITCHDEV NIC's VF. e.g. "0000:03:02.3".
* `vlan` (int, optional): VLAN ID to assign for the VF. Value must be in the range 0-4094 (0 for disabled, 1-4094 for valid VLAN IDs).
* `mac` (string, optional): MAC address to assign for the VF


An Accelerated Bridge CNI config with each field filled out looks like:

```json
{
    "cniVersion": "0.3.1",
    "name": "some-net",
    "type": "accelerated-bridge",
    "deviceID": "0000:03:02.0",
    "vlan": 1000,
    "mac": "CA:FE:C0:FF:EE:00"
}
```

### Runtime Configuration

The Accelerated Bridge CNI accepts a MAC address when passed as a runtime configuration - that is as part of a Kubernetes Pod spec. An example pod with a runtime configuration is:

```
apiVersion: v1
kind: Pod
metadata:
  name: samplepod
  annotations:
    k8s.v1.cni.cncf.io/networks: '[
      {
        "name": "some-net",
        "mac": "CA:FE:C0:FF:EE:00"
      }
    ]'
spec:
  containers:
  - name: runTimeConfig
    command: ["/bin/bash", "-c", "sleep 300"]
    image: centos/tools

```

The above config will configure a VF of type "some-net" with the MAC address configured as the value supplied under the 'k8s.v1.cni.cncf.io/networks'. Where the MAC address supplied is invalid the container may be created with an unexpected address.

To avoid this it's key to ensure the supplied MAC is valid for the specified interface. On some systems setting a Multicast MAC address (Where the least significant bit of the first octet is '1') results in failure to set the MAC address.
