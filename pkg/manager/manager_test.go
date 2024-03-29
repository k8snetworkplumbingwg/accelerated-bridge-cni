package manager

import (
	"errors"
	"net"
	"os"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"

	mgrMocks "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/manager/mocks"
	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
	utilsMocks "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/utils/mocks"
)

// Fake NS - implements ns.NetNS interface
type fakeNetNS struct {
	closed bool
	fd     uintptr
	path   string
}

func (f *fakeNetNS) Do(toRun func(ns.NetNS) error) error {
	return toRun(f)
}

func (f *fakeNetNS) Set() error {
	return nil
}

func (f *fakeNetNS) Path() string {
	return f.path
}

func (f *fakeNetNS) Fd() uintptr {
	return f.fd
}

func (f *fakeNetNS) Close() error {
	f.closed = true
	return nil
}

func newFakeNs() ns.NetNS {
	return &fakeNetNS{
		closed: false,
		fd:     17,
		path:   "/proc/4123/ns/net",
	}
}

// FakeLink is a dummy netlink struct used during testing
type FakeLink struct {
	netlink.LinkAttrs
}

func (l *FakeLink) Attrs() *netlink.LinkAttrs {
	return &l.LinkAttrs
}

func (l *FakeLink) Type() string {
	return "FakeLink"
}

type FakeBondLink struct {
	netlink.LinkAttrs
}

func (l *FakeBondLink) Attrs() *netlink.LinkAttrs {
	return &l.LinkAttrs
}

func (l *FakeBondLink) Type() string {
	return "bond"
}

var _ = Describe("IPCLock", func() {
	Context("Checking Lock/Unlock functions", func() {
		testpath := "/tmp/accel-br-locktest/"
		BeforeEach(func() {
			err := os.MkdirAll(testpath, 0700)
			check(err)
		})
		AfterEach(func() {
			err := os.RemoveAll(testpath)
			check(err)
		})
		It("Lock/Unlock file with existing path (success)", func() {
			lock := NewIPCLock(testpath + "flock.lock")
			err1 := lock.Lock()
			err2 := lock.Unlock()
			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())
		})
		It("Lock/Unlock file with new path (success)", func() {
			lock := NewIPCLock(testpath + "subdir/flock.lock")
			err1 := lock.Lock()
			err2 := lock.Unlock()
			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())
		})
		It("Lock file with existing path but no permission (failed)", func() {
			err := os.Chmod(testpath, 0400)
			check(err)
			lock := NewIPCLock(testpath + "flock.lock")
			err1 := lock.Lock()
			Expect(err1).To(HaveOccurred())
		})
		It("Lock with new path but no permission (failed)", func() {
			err := os.Chmod(testpath, 0400)
			check(err)
			lock := NewIPCLock(testpath + "subdir/flock.lock")
			err1 := lock.Lock()
			Expect(err1).To(HaveOccurred())
		})
		It("Unlock non-existing file with existing path (should be a no-op) (success)", func() {
			lock := NewIPCLock(testpath + "flock.lock")
			err1 := lock.Unlock()
			Expect(err1).ToNot(HaveOccurred())
		})
		It("Unlock non-existing file with non-existing path (should be a no-op) (success)", func() {
			lock := NewIPCLock(testpath + "subdir/flock.lock")
			err1 := lock.Unlock()
			Expect(err1).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("Manager", func() {
	var (
		t GinkgoTInterface
	)
	BeforeEach(func() {
		t = GinkgoT()
	})

	Context("Checking SetupVF function", func() {
		var (
			podifName string
			contID    string
			netconf   *types.PluginConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
				},
				PFName:      "enp175s0f1",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName: "enp175s6",
				},
			}
			t = GinkgoT()
		})

		It("Assuming existing interface", func() {
			targetNetNS := newFakeNs()
			mocked := &utilsMocks.Netlink{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")

			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			m := manager{nLink: mocked}
			macAddr, err := m.SetupVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(macAddr).To(Equal("6e:16:06:0e:b7:e9"))
			mocked.AssertExpectations(t)
		})
		It("Setting mac address", func() {
			targetNetNS := newFakeNs()
			mocked := &utilsMocks.Netlink{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			netconf.MAC = "e4:11:22:33:44:55"
			expMac, err := net.ParseMAC(netconf.MAC)
			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetHardwareAddr", fakeLink, expMac).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			m := manager{nLink: mocked}
			macAddr, err := m.SetupVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(macAddr).To(Equal(netconf.MAC))
			mocked.AssertExpectations(t)
		})
		It("Setting mtu", func() {
			targetNetNS := newFakeNs()
			mocked := &utilsMocks.Netlink{}
			netconf.MTU = 2000
			origMTU := 1500
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index: 1000,
				Name:  "dummylink",
				MTU:   origMTU,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, netconf.MTU).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			m := manager{nLink: mocked}
			_, err := m.SetupVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(netconf.OrigVfState.MTU).To(Equal(origMTU))
			mocked.AssertExpectations(t)
		})
	})

	Context("Checking ReleaseVF function", func() {
		var (
			podifName string
			contID    string
			netconf   *types.PluginConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
				},
				PFName:      "enp175s0f1",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Assuming existing interface", func() {
			targetNetNS := newFakeNs()
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.ContIFNames).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			m := manager{nLink: mocked}
			err := m.ReleaseVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ReleaseVF function - restore config", func() {
		var (
			podifName  string
			contID     string
			netconf    *types.PluginConf
			origMTU    int
			hostIFName string
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			origMTU = 1500
			hostIFName = "enp175s6"
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
				},
				PFName:      "enp175s0f1",
				VFID:        0,
				MAC:         "aa:f3:8d:65:1b:d4",
				MTU:         1600,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName:   hostIFName,
					EffectiveMAC: "c6:c8:7f:1f:21:90",
					MTU:          origMTU,
				},
			}
		})
		It("Restores Effective MAC address and MTU when provided in netconf", func() {
			targetNetNS := newFakeNs()
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.ContIFNames).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, origMTU).Return(nil)
			origEffMac, err := net.ParseMAC(netconf.OrigVfState.EffectiveMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetHardwareAddr", fakeLink, origEffMac).Return(nil)
			m := manager{nLink: mocked}
			err = m.ReleaseVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ResetVFConfig function - restore config no user params", func() {
		var (
			netconf *types.PluginConf
		)

		BeforeEach(func() {
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
				},
				PFName:      "enp175s0f1",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Does not change VF config if it wasnt requested to be changed in netconf", func() {
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.PFName).Return(fakeLink, nil)
			m := manager{nLink: mocked}
			err := m.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})

	Context("Checking ResetVFConfig function - restore config with user params", func() {
		var (
			netconf *types.PluginConf
		)

		BeforeEach(func() {
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
					Vlan:     6,
				},
				PFName:      "enp175s0f1",
				VFID:        3,
				ContIFNames: "net1",
				MAC:         "d2:fc:22:a7:0d:e8",
				OrigVfState: types.VfState{
					HostIFName:   "enp175s6",
					AdminMAC:     "aa:f3:8d:65:1b:d4",
					EffectiveMAC: "aa:f3:8d:65:1b:d4",
				},
			}
		})
		It("Restores original VF configurations", func() {
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.PFName).Return(fakeLink, nil)
			origMac, err := net.ParseMAC(netconf.OrigVfState.AdminMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetVfHardwareAddr", fakeLink, netconf.VFID, origMac).Return(nil)

			m := manager{nLink: mocked}
			err = m.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking AttachRepresentor function", func() {
		var (
			netconf *types.PluginConf
		)

		BeforeEach(func() {
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
					Vlan:     100,
				},
				Representor:  "dummylink",
				PFName:       "enp175s0f1",
				ActualBridge: "bridge1",
				VFID:         0,
				Trunk:        []int{4, 6},
			}
			// Mute logger
			zerolog.SetGlobalLevel(zerolog.Disabled)
		})
		It("Attaching dummy link to the bridge (success)", func() {
			origMtu := 1500
			newMtu := 2000
			netconf.MTU = newMtu
			mockedNl := &utilsMocks.Netlink{}
			mockedSr := &utilsMocks.Sriovnet{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 0,
				MTU:         origMtu,
			}}

			mockedNl.On("LinkByName", netconf.ActualBridge).Return(fakeBridge, nil)
			mockedNl.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mockedSr.On("GetVfRepresentor", netconf.PFName, netconf.VFID).Return(fakeLink.Name, nil)
			mockedNl.On("LinkSetUp", fakeLink).Return(nil)
			mockedNl.On("LinkSetMaster", fakeLink, fakeBridge).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				bridge := args.Get(1).(netlink.Link)
				link.Attrs().MasterIndex = bridge.Attrs().Index
			}).Return(nil)
			mockedNl.On("BridgeVlanDel", fakeLink, uint16(1), true, true, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(100), true, true, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(4), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(6), false, false, false, true).Return(nil)
			mockedNl.On("LinkSetMTU", fakeLink, newMtu).Return(nil)

			m := manager{nLink: mockedNl, sriov: mockedSr}
			err := m.AttachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(fakeBridge.Attrs().Index))
			mockedNl.AssertExpectations(t)
			mockedSr.AssertExpectations(t)
			Expect(netconf.OrigRepState.MTU).To(Equal(origMtu))
		})
		It("Attaching dummy link to the bridge (failure)", func() {
			mockedNl := &utilsMocks.Netlink{}
			mockedSr := &utilsMocks.Sriovnet{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 0,
			}}

			mockedNl.On("LinkByName", netconf.ActualBridge).Return(fakeBridge, nil)
			mockedNl.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mockedSr.On("GetVfRepresentor", netconf.PFName, netconf.VFID).Return(fakeLink.Name, nil)
			mockedNl.On("LinkSetUp", fakeLink).Return(nil)
			mockedNl.On("LinkSetMaster", fakeLink, fakeBridge).Return(errors.New("some error"))

			m := manager{nLink: mockedNl, sriov: mockedSr}
			err := m.AttachRepresentor(netconf)
			Expect(err).To(HaveOccurred())
			mockedNl.AssertExpectations(t)
			mockedSr.AssertExpectations(t)
		})
		It("Attaching dummy link to the bridge setting uplink vlans (success)", func() {
			origMtu := 1500
			newMtu := 2000
			netconf.MTU = newMtu
			netconf.SetUplinkVlan = true
			mockedNl := &utilsMocks.Netlink{}
			mockedSr := &utilsMocks.Sriovnet{}
			mockedLock := &mgrMocks.IPCLock{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 0,
				MTU:         origMtu,
			}}
			fakeUpLink := &FakeLink{netlink.LinkAttrs{
				Name:        "enp175s0f1",
				MasterIndex: 1000,
				MTU:         origMtu,
			}}

			mockedNl.On("LinkByName", netconf.ActualBridge).Return(fakeBridge, nil)
			mockedNl.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mockedSr.On("GetVfRepresentor", netconf.PFName, netconf.VFID).Return(fakeLink.Name, nil)
			mockedNl.On("LinkSetUp", fakeLink).Return(nil)
			mockedNl.On("LinkSetMaster", fakeLink, fakeBridge).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				bridge := args.Get(1).(netlink.Link)
				link.Attrs().MasterIndex = bridge.Attrs().Index
			}).Return(nil)
			mockedNl.On("BridgeVlanDel", fakeLink, uint16(1), true, true, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(100), true, true, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(4), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(6), false, false, false, true).Return(nil)
			mockedNl.On("LinkSetMTU", fakeLink, newMtu).Return(nil)

			// addUplinkVlans function
			mockedNl.On("LinkByName", netconf.PFName).Return(fakeUpLink, nil)
			// link is not part of a bond
			mockedNl.On("LinkByIndex", fakeUpLink.Attrs().MasterIndex).Return(fakeBridge, nil)
			mockedLock.On("Lock").Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeUpLink, uint16(100), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeUpLink, uint16(4), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeUpLink, uint16(6), false, false, false, true).Return(nil)
			mockedLock.On("Unlock").Return(nil)

			m := manager{nLink: mockedNl, sriov: mockedSr, vlanUplinkLock: mockedLock}
			err := m.AttachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeUpLink.Attrs().MasterIndex).To(Equal(fakeBridge.Attrs().Index))
			mockedNl.AssertExpectations(t)
			mockedSr.AssertExpectations(t)
		})
		It("Attaching dummy link to the bridge setting bond uplink vlans (success)", func() {
			origMtu := 1500
			newMtu := 2000
			netconf.MTU = newMtu
			netconf.SetUplinkVlan = true
			mockedNl := &utilsMocks.Netlink{}
			mockedSr := &utilsMocks.Sriovnet{}
			mockedLock := &mgrMocks.IPCLock{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 0,
				MTU:         origMtu,
			}}
			fakeUpLink := &FakeLink{netlink.LinkAttrs{
				Name:        "enp175s0f1",
				Index:       10,
				MasterIndex: 20,
				MTU:         origMtu,
			}}
			fakeBondUpLink := &FakeBondLink{netlink.LinkAttrs{
				Name:        "bond0",
				Index:       20,
				MasterIndex: 1000,
				MTU:         origMtu,
			}}

			mockedNl.On("LinkByName", netconf.ActualBridge).Return(fakeBridge, nil)
			mockedNl.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mockedSr.On("GetVfRepresentor", netconf.PFName, netconf.VFID).Return(fakeLink.Name, nil)
			mockedNl.On("LinkSetUp", fakeLink).Return(nil)
			mockedNl.On("LinkSetMaster", fakeLink, fakeBridge).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				bridge := args.Get(1).(netlink.Link)
				link.Attrs().MasterIndex = bridge.Attrs().Index
			}).Return(nil)
			mockedNl.On("BridgeVlanDel", fakeLink, uint16(1), true, true, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(100), true, true, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(4), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeLink, uint16(6), false, false, false, true).Return(nil)
			mockedNl.On("LinkSetMTU", fakeLink, newMtu).Return(nil)

			// addUplinkVlans function
			mockedNl.On("LinkByName", netconf.PFName).Return(fakeUpLink, nil)
			// link is part of a bond
			mockedNl.On("LinkByIndex", fakeUpLink.Attrs().MasterIndex).Return(fakeBondUpLink, nil)
			mockedLock.On("Lock").Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeBondUpLink, uint16(100), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeBondUpLink, uint16(4), false, false, false, true).Return(nil)
			mockedNl.On("BridgeVlanAdd", fakeBondUpLink, uint16(6), false, false, false, true).Return(nil)
			mockedLock.On("Unlock").Return(nil)

			m := manager{nLink: mockedNl, sriov: mockedSr, vlanUplinkLock: mockedLock}
			err := m.AttachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBondUpLink.Attrs().MasterIndex).To(Equal(fakeBridge.Attrs().Index))
			mockedNl.AssertExpectations(t)
			mockedSr.AssertExpectations(t)
		})
	})
	Context("Checking DetachRepresentor function", func() {
		var (
			netconf *types.PluginConf
		)
		origMtu := 1500
		newMtu := 2000

		BeforeEach(func() {
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
					Vlan:     100,
				},
				Representor: "dummylink",
				PFName:      "enp175s0f1",
				VFID:        0,
				MTU:         newMtu,
				OrigRepState: types.RepState{
					MTU: origMtu,
				},
				Trunk: []int{4, 6},
			}
			// Mute logger
			zerolog.SetGlobalLevel(zerolog.Disabled)
		})
		It("Detaching dummy link from the bridge (success)", func() {
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 1000,
			}}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				link.Attrs().MasterIndex = 0
			}).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, origMtu).Return(nil)

			m := manager{nLink: mocked}
			err := m.DetachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(0))
			mocked.AssertExpectations(t)
		})
		It("Detaching dummy link from the bridge and removing uplink vlans (success)", func() {
			netconf.SetUplinkVlan = true
			mocked := &utilsMocks.Netlink{}
			mockedLock := &mgrMocks.IPCLock{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				Index:       10,
				MasterIndex: 1000,
			}}
			fakeUpLink := &FakeLink{netlink.LinkAttrs{
				Name:        "enp175s0f1",
				Index:       20,
				MasterIndex: 1000,
				MTU:         origMtu,
			}}
			fakeVlanInfo := map[int32][]*nl.BridgeVlanInfo{
				10: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 4},
					{Flags: 0, Vid: 6}},
				20: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 4},
					{Flags: 0, Vid: 6}},
			}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				link.Attrs().MasterIndex = 0
			}).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, origMtu).Return(nil)

			// deleteUplinkVlans function
			mocked.On("LinkByName", netconf.PFName).Return(fakeUpLink, nil)
			// link is not part of a bond
			mocked.On("LinkByIndex", fakeUpLink.Attrs().MasterIndex).Return(fakeBridge, nil)
			mockedLock.On("Lock").Return(nil)
			mocked.On("LinkList").Return([]netlink.Link{fakeLink, fakeUpLink}, nil)
			mocked.On("BridgeVlanList").Return(fakeVlanInfo, nil)
			mocked.On("BridgeVlanDel", fakeUpLink, uint16(100), false, false, false, true).Return(nil)
			mocked.On("BridgeVlanDel", fakeUpLink, uint16(4), false, false, false, true).Return(nil)
			mocked.On("BridgeVlanDel", fakeUpLink, uint16(6), false, false, false, true).Return(nil)
			mockedLock.On("Unlock").Return(nil)

			m := manager{nLink: mocked, vlanUplinkLock: mockedLock}
			err := m.DetachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(0))
			mocked.AssertExpectations(t)
		})
		It("Detaching dummy link from the bridge and removing bond uplink vlans (success)", func() {
			netconf.SetUplinkVlan = true
			mocked := &utilsMocks.Netlink{}
			mockedLock := &mgrMocks.IPCLock{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				Index:       10,
				MasterIndex: 1000,
			}}
			fakeUpLink := &FakeLink{netlink.LinkAttrs{
				Name:        "enp175s0f1",
				Index:       20,
				MasterIndex: 30,
				MTU:         origMtu,
			}}
			fakeBondUpLink := &FakeBondLink{netlink.LinkAttrs{
				Name:        "bond0",
				Index:       30,
				MasterIndex: 1000,
				MTU:         origMtu,
			}}
			fakeVlanInfo := map[int32][]*nl.BridgeVlanInfo{
				10: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 4},
					{Flags: 0, Vid: 6}},
				30: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 4},
					{Flags: 0, Vid: 6}},
			}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				link.Attrs().MasterIndex = 0
			}).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, origMtu).Return(nil)

			// deleteUplinkVlans function
			mocked.On("LinkByName", netconf.PFName).Return(fakeUpLink, nil)
			// link is part of a bond
			mocked.On("LinkByIndex", fakeUpLink.Attrs().MasterIndex).Return(fakeBondUpLink, nil)
			mocked.On("LinkByIndex", fakeBondUpLink.Attrs().MasterIndex).Return(fakeBridge, nil)
			mockedLock.On("Lock").Return(nil)
			mocked.On("LinkList").Return([]netlink.Link{fakeLink, fakeBondUpLink}, nil)
			mocked.On("BridgeVlanList").Return(fakeVlanInfo, nil)
			mocked.On("BridgeVlanDel", fakeBondUpLink, uint16(100), false, false, false, true).Return(nil)
			mocked.On("BridgeVlanDel", fakeBondUpLink, uint16(4), false, false, false, true).Return(nil)
			mocked.On("BridgeVlanDel", fakeBondUpLink, uint16(6), false, false, false, true).Return(nil)
			mockedLock.On("Unlock").Return(nil)

			m := manager{nLink: mocked, vlanUplinkLock: mockedLock}
			err := m.DetachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(0))
			mocked.AssertExpectations(t)
		})
		It("Detaching dummy link from the bridge and removing bond uplink vlans with 2 in use (success)", func() {
			netconf.SetUplinkVlan = true
			mocked := &utilsMocks.Netlink{}
			mockedLock := &mgrMocks.IPCLock{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				Index:       10,
				MasterIndex: 1000,
			}}
			fakeLinkOther := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				Index:       15,
				MasterIndex: 1000,
			}}
			fakeUpLink := &FakeLink{netlink.LinkAttrs{
				Name:        "enp175s0f1",
				Index:       20,
				MasterIndex: 30,
				MTU:         origMtu,
			}}
			fakeBondUpLink := &FakeBondLink{netlink.LinkAttrs{
				Name:        "bond0",
				Index:       30,
				MasterIndex: 1000,
				MTU:         origMtu,
			}}
			fakeVlanInfo := map[int32][]*nl.BridgeVlanInfo{
				10: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 4},
					{Flags: 0, Vid: 6}},
				15: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 6}},
				30: {{Flags: 0, Vid: 100},
					{Flags: 0, Vid: 4},
					{Flags: 0, Vid: 6}},
			}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				link.Attrs().MasterIndex = 0
			}).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, origMtu).Return(nil)

			// deleteUplinkVlans function
			mocked.On("LinkByName", netconf.PFName).Return(fakeUpLink, nil)
			// link is part of a bond
			mocked.On("LinkByIndex", fakeUpLink.Attrs().MasterIndex).Return(fakeBondUpLink, nil)
			mocked.On("LinkByIndex", fakeBondUpLink.Attrs().MasterIndex).Return(fakeBridge, nil)
			mockedLock.On("Lock").Return(nil)
			mocked.On("LinkList").Return([]netlink.Link{fakeLink, fakeLinkOther, fakeBondUpLink}, nil)
			mocked.On("BridgeVlanList").Return(fakeVlanInfo, nil)
			mocked.On("BridgeVlanDel", fakeBondUpLink, uint16(4), false, false, false, true).Return(nil)
			mockedLock.On("Unlock").Return(nil)

			m := manager{nLink: mocked, vlanUplinkLock: mockedLock}
			err := m.DetachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(0))
			mocked.AssertExpectations(t)
		})
		It("Deleting uplink vlans for bond not part of a bridge (failure)", func() {
			netconf.SetUplinkVlan = true
			mocked := &utilsMocks.Netlink{}
			fakeUpLink := &FakeLink{netlink.LinkAttrs{
				Name:        "enp175s0f1",
				Index:       20,
				MasterIndex: 30,
				MTU:         origMtu,
			}}
			fakeBondUpLink := &FakeBondLink{netlink.LinkAttrs{
				Name:        "bond0",
				Index:       30,
				MasterIndex: 0,
				MTU:         origMtu,
			}}

			// deleteUplinkVlans function
			mocked.On("LinkByName", netconf.PFName).Return(fakeUpLink, nil)
			// link is part of a bond but that bond is not part of the bridge!
			mocked.On("LinkByIndex", fakeUpLink.Attrs().MasterIndex).Return(fakeBondUpLink, nil)
			// bond has no master, GetParentBridgeForLink will fail

			m := manager{nLink: mocked}
			err := m.deleteUplinkVlans(netconf)
			Expect(err).To(HaveOccurred())
			mocked.AssertExpectations(t)
		})
		It("Detaching dummy link from the bridge (failure)", func() {
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 1000,
			}}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetMTU", fakeLink, origMtu).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Return(errors.New("some error"))

			m := manager{nLink: mocked}
			err := m.DetachRepresentor(netconf)
			Expect(err).To(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ApplyVF function - persist original VF admin MAC", func() {
		var (
			netconf *types.PluginConf
		)
		origMac, _ := net.ParseMAC("d2:fc:22:a7:0d:e8")
		newMac, _ := net.ParseMAC("d2:fc:22:a7:0d:e8")

		BeforeEach(func() {
			netconf = &types.PluginConf{
				NetConf: types.NetConf{
					DeviceID: "0000:af:06.0",
					Vlan:     6,
				},
				PFName:      "enp175s0f1",
				VFID:        3,
				ContIFNames: "net1",
				MAC:         newMac.String(),
				OrigVfState: types.VfState{},
			}
		})
		It("Restores original VF configurations", func() {
			mocked := &utilsMocks.Netlink{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Vfs: []netlink.VfInfo{{ID: 3, Mac: origMac}}}}

			mocked.On("LinkByName", netconf.PFName).Return(fakeLink, nil)

			mocked.On("LinkSetVfHardwareAddr", fakeLink, netconf.VFID, newMac).Return(nil)

			m := manager{nLink: mocked}
			err := m.ApplyVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
			Expect(netconf.OrigVfState.AdminMAC).To(Equal(origMac.String()))
		})
	})
})
