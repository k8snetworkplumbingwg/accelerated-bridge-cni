## Configuration reference - Accelerated Bridge CNI

The Accelerated Bridge CNI configures networks through a CNI spec configuration object. In a Kubernetes cluster set up with Multus this object is most often delivered as a Network Attachment Definition.


### Parameters
* `name` (string, required): the name of the network
* `type` (string, required): "accelerated-bridge"
* `ipam` (dictionary, optional): IPAM configuration to be used for this network.
* `deviceID` (string, required): A valid pci address of a SWITCHDEV NIC's VF. e.g. "0000:03:02.3".
* `debug` (bool, optional): Enable verbose logging
* `bridge` (string, optional): single or comma separated list of linux bridges to use e.g. `br1` or `br1, br2`, default value is `cni0`.
  CNI will use automatic bridge selection logic if multiple bridges are set.
* `vlan` (int, optional): VLAN ID to assign for the VF. Value must be in the range 0-4094 (0 for disabled, 1-4094 for valid VLAN IDs).
* `mac` (string, optional): MAC address to assign for the VF
* `mtu` (int, optional): MTU configuration for the VF.
* `trunk` (array, optional): VLAN trunk configuration for the VF. 
  Value must be an array of objects with trunk config, e.g.
  `[{"id": 42}, {"minID": 100, "maxID": 105}, {"id": 198, "minID": 200, "maxID": 210}]`,
  which means that trunk will allow folowing VLANs 42,100-105,198,200-210
* `runtimeConfig` (dictionary, optional): CNI RuntimeConfig,
  `runtimeConfig.mac` is the only supported option for now, it takes precedence over top-level `mac` option;
  e.g. `runtimeConfig: {"mac": "CA:FE:C0:FF:EE:00"}`


Default VLAN (1) will be used for VF if `vlan` and `trunk` options are not configured.
In this case bridge expect untagged frames from VF and will drop all tagged frames.


It is also possible to use `vlan` and `trunk` options together. 
In this case, VLAN ID from `vlan` option will be used by the bridge as
native VLAN for VF. This means that bridge will add tag from `vlan` option to
all untagged frames from VF and allow VF to send and receive tagged frames with tags from `trunk` option.


_Note: The CNI assumes the bridge is present and configured. 
It does not manage other bridge configuration (e.g vlan_filtering option) or any uplink configurations._


An Accelerated Bridge CNI config with each field filled out looks like:

```json
{
    "cniVersion": "0.3.1",
    "name": "some-net",
    "type": "accelerated-bridge",
    "ipam": {
      "type": "host-local",
      "subnet": "10.56.217.0/24",
      "routes": [{
        "dst": "0.0.0.0/0"
      }],
      "gateway": "10.56.217.1"
    },
    "debug": true,
    "bridge": "br1,br2",
    "deviceID": "0000:03:02.0",
    "mac": "CA:FE:C0:FF:EE:00",
    "vlan": 1000,
    "mtu": 2000,
    "trunk": [{"minID": 100, "maxID": 105}],
    "runtimeConfig": {
      "mac": "CA:FE:C0:FF:EE:11"
    }
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
