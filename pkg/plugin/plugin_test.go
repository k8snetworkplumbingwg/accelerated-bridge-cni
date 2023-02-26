package plugin

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/stretchr/testify/mock"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/cache"
	cacheMocks "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/cache/mocks"
	configMocks "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/config/mocks"
	managerMocks "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/manager/mocks"
	pluginMocks "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/plugin/mocks"
	localtypes "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testValidDeviceID   = "0000:af:06.1"
	testValidName       = "mynet"
	testValidBridge     = "br1"
	testValidVlan       = 100
	testValidTrunkID    = 42
	testValidTrunkMinID = 1005
	testValidTrunkMaxID = 1010
	testValidTrunk      = []localtypes.Trunk{{
		ID:    &testValidTrunkID,
		MinID: &testValidTrunkMinID,
		MaxID: &testValidTrunkMaxID}}
	testValidIPAM                       = types.IPAM{Type: "host-local"}
	testValidTrunkInt                   = []int{42, 1005, 1006, 1007, 1008, 1009, 1010}
	testValidPFName                     = "ens1f0np0"
	testValidVFID                       = 0
	testValidRepName                    = "eth5"
	testValidContIFNames                = "net1"
	testValidContainerID                = "a1b2c3d4e5f6"
	testValidNSPath                     = "/proc/12444/ns/net"
	testValidMAC                        = "b3:ec:90:4c:5b:11"
	testValidMAC2                       = "b3:ec:90:4c:5b:12"
	testValidMAC3                       = "b3:ec:90:4c:5b:13"
	testValidCacheRef    cache.StateRef = "/var/lib/cni/accelerated-bridge/mynet-a1b2c3d4e5f6-net1"
	errTest                             = errors.New("test err")
)

func getValidPluginConf() *localtypes.PluginConf {
	return &localtypes.PluginConf{
		NetConf: localtypes.NetConf{
			NetConf:  types.NetConf{CNIVersion: "0.4.0", IPAM: testValidIPAM, Name: testValidName},
			Bridge:   testValidBridge,
			Debug:    true,
			DeviceID: testValidDeviceID,
			Vlan:     testValidVlan,
			Trunk:    testValidTrunk,
		},
		Trunk:        testValidTrunkInt,
		PFName:       testValidPFName,
		ActualBridge: testValidBridge,
		ContIFNames:  testValidContIFNames,
		VFID:         testValidVFID,
		Representor:  testValidRepName,
	}
}

func getValidCmdArgs() *skel.CmdArgs {
	return &skel.CmdArgs{
		Netns:       testValidNSPath,
		ContainerID: testValidContainerID,
		IfName:      testValidContIFNames,
		StdinData:   []byte("data"),
	}
}

func getValidIPAMResult() types.Result {
	_, ipNet, _ := net.ParseCIDR("192.168.100.101/24")
	ipConf := current.IPConfig{
		Address: *ipNet,
		Gateway: net.ParseIP("192.168.100.1"),
	}
	return &current.Result{
		CNIVersion: "1.0.0",
		IPs:        []*current.IPConfig{&ipConf},
	}
}

var _ = Describe("Plugin - test CNI command flows", func() {
	var (
		t           GinkgoTInterface
		plugin      Plugin
		nsMock      *pluginMocks.NS
		ipamMock    *pluginMocks.IPAM
		cacheMock   *cacheMocks.StateCache
		managerMock *managerMocks.Manager
		configMock  *configMocks.Loader
		netNSMock   *pluginMocks.NetNS
		pluginConf  *localtypes.PluginConf
		cmdArgs     *skel.CmdArgs
	)

	JustBeforeEach(func() {
		t = GinkgoT()
		nsMock = &pluginMocks.NS{}
		ipamMock = &pluginMocks.IPAM{}
		cacheMock = &cacheMocks.StateCache{}
		managerMock = &managerMocks.Manager{}
		configMock = &configMocks.Loader{}
		netNSMock = &pluginMocks.NetNS{}
		plugin = Plugin{
			netNS:   nsMock,
			ipam:    ipamMock,
			manager: managerMock,
			config:  configMock,
			cache:   cacheMock,
		}
		pluginConf = getValidPluginConf()
		cmdArgs = getValidCmdArgs()
	})

	JustAfterEach(func() {
		nsMock.AssertExpectations(t)
		ipamMock.AssertExpectations(t)
		cacheMock.AssertExpectations(t)
		managerMock.AssertExpectations(t)
		configMock.AssertExpectations(t)
		netNSMock.AssertExpectations(t)
	})

	Describe("CmdAdd", func() {
		successfullyParseConfig := func(_ bool) {
			configMock.On("ParseConf", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				*args[1].(*localtypes.PluginConf) = *pluginConf
			}).Return(nil).Once()
		}
		successfullyGetNS := func(withDeps bool) {
			if withDeps {
				successfullyParseConfig(true)
			}
			nsMock.On("GetNS", testValidNSPath).Return(netNSMock, nil).Once()
			netNSMock.On("Path").Return(testValidNSPath).Once()
		}
		successfullyAttachRepresentor := func(withDeps bool) {
			if withDeps {
				successfullyGetNS(true)
			}
			managerMock.On("AttachRepresentor", pluginConf).Return(nil).Once()
		}
		successfullyApplyVFConfig := func(withDeps bool) {
			if withDeps {
				successfullyAttachRepresentor(true)
			}
			managerMock.On("ApplyVFConfig", pluginConf).Return(nil).Once()
		}
		successfullySetupVF := func(withDeps bool) {
			if withDeps {
				successfullyApplyVFConfig(true)
			}
			managerMock.On("SetupVF",
				pluginConf, testValidContIFNames, testValidContainerID, netNSMock).
				Return(testValidMAC, nil).Once()
		}
		successfullyExecAdd := func(withDeps bool) {
			if withDeps {
				successfullySetupVF(true)
			}
			ipamMock.On("ExecAdd", pluginConf.IPAM.Type, cmdArgs.StdinData).
				Return(getValidIPAMResult(), nil).Once()
		}
		successfullyConfigureIface := func(withDeps bool) {
			if withDeps {
				successfullyExecAdd(true)
			}
			ipamMock.On("ConfigureIface", cmdArgs.IfName,
				mock.MatchedBy(func(conf *current.Result) bool {
					return len(conf.Interfaces) > 0 && conf.Interfaces[0].Mac == testValidMAC
				})).
				Return(nil).Once()
			netNSMock.On("Do", mock.Anything).Return(func(f func(ns.NetNS) error) error {
				return f(nil)
			}).Once()
		}
		configureCacheMock := func() {
			cacheMock.On("GetStateRef", pluginConf.Name, cmdArgs.ContainerID, cmdArgs.IfName).
				Return(testValidCacheRef).Once()
			cacheMock.On("Save", testValidCacheRef, pluginConf).
				Return(nil).Once()
		}
		successfullySave := func(withDeps bool) {
			if withDeps {
				successfullyConfigureIface(true)
			}
			configureCacheMock()
		}
		cleanupGetNS := func() {
			netNSMock.On("Close").Return(nil).Once()
		}
		cleanupAttachRepresentor := func() {
			cleanupGetNS()
			managerMock.On("DetachRepresentor", pluginConf).Return(nil).Once()
		}
		cleanupSetupVFConfig := func() {
			cleanupAttachRepresentor()
			netNSMock.On("Do", mock.Anything).Return(nil).Once()
			managerMock.On("ReleaseVF",
				pluginConf, testValidContIFNames, testValidContainerID, netNSMock).Return(nil).Once()
		}
		cleanupExecAdd := func() {
			cleanupSetupVFConfig()
			ipamMock.On("ExecDel", pluginConf.IPAM.Type, cmdArgs.StdinData).Return(nil).Once()
		}
		Context("Failed scenarios", func() {
			It("Fail to parse config", func() {
				configMock.On("ParseConf", mock.Anything, mock.Anything).Return(errTest).Once()
				Expect(plugin.CmdAdd(getValidCmdArgs())).To(HaveOccurred())
			})
			It("Failed to get NS", func() {
				successfullyParseConfig(true)
				nsMock.On("GetNS", testValidNSPath).Return(nil, errTest).Once()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to attach representor", func() {
				successfullyGetNS(true)
				managerMock.On("AttachRepresentor", pluginConf).Return(errTest).Once()
				cleanupGetNS()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to ApplyVFConfig", func() {
				successfullyAttachRepresentor(true)
				managerMock.On("ApplyVFConfig", pluginConf).Return(errTest).Once()
				cleanupAttachRepresentor()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to SetupVF", func() {
				successfullyApplyVFConfig(true)
				managerMock.On("SetupVF",
					pluginConf, testValidContIFNames, testValidContainerID, netNSMock).
					Return("", errTest).Once()
				cleanupSetupVFConfig()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to IPAM Add", func() {
				successfullySetupVF(true)
				ipamMock.On("ExecAdd", pluginConf.IPAM.Type, cmdArgs.StdinData).
					Return(nil, errTest).Once()
				cleanupSetupVFConfig()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("IPAM add returns no IPs", func() {
				successfullySetupVF(true)
				result := getValidIPAMResult()
				result.(*current.Result).IPs = nil
				ipamMock.On("ExecAdd", pluginConf.IPAM.Type, cmdArgs.StdinData).
					Return(result, nil).Once()
				cleanupExecAdd()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to IPAM ConfigureIface", func() {
				successfullyExecAdd(true)
				ipamMock.On("ConfigureIface", cmdArgs.IfName, mock.Anything).
					Return(errTest).Once()
				netNSMock.On("Do", mock.Anything).Return(func(f func(ns.NetNS) error) error {
					return f(nil)
				}).Once()
				cleanupExecAdd()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
			It("Failed save cache", func() {
				successfullyConfigureIface(true)
				cacheMock.On("GetStateRef", pluginConf.Name, cmdArgs.ContainerID, cmdArgs.IfName).
					Return(testValidCacheRef).Once()
				cacheMock.On("Save", testValidCacheRef, pluginConf).
					Return(errTest).Once()
				cleanupExecAdd()
				Expect(plugin.CmdAdd(cmdArgs)).To(HaveOccurred())
			})
		})
		Context("Successful scenarios", func() {
			It("with IPAM", func() {
				successfullySave(true)
				cleanupGetNS()
				Expect(plugin.CmdAdd(cmdArgs)).ToNot(HaveOccurred())
			})
			It("no IPAM", func() {
				pluginConf.IPAM = types.IPAM{}
				successfullySetupVF(true)
				configureCacheMock()
				cleanupGetNS()
				Expect(plugin.CmdAdd(cmdArgs)).ToNot(HaveOccurred())
			})
			It("userspace driver", func() {
				pluginConf.IsUserspaceDriver = true
				successfullyApplyVFConfig(true)
				successfullyExecAdd(false)
				successfullySave(false)
				cleanupGetNS()
				Expect(plugin.CmdAdd(cmdArgs)).ToNot(HaveOccurred())
			})
		})
		Context("MAC address configuration", func() {

			var updatedPluginConf *localtypes.PluginConf

			JustBeforeEach(func() {
				successfullyGetNS(true)
				cleanupGetNS()
				// workaround to access pluginConf
				managerMock.On("AttachRepresentor", mock.Anything).Run(func(args mock.Arguments) {
					updatedPluginConf = args[0].(*localtypes.PluginConf)
				}).Return(errTest).Once()
			})
			It("top-level MAC options should work", func() {
				pluginConf.MAC = testValidMAC
				_ = plugin.CmdAdd(cmdArgs)
				Expect(updatedPluginConf.MAC).To(BeIdenticalTo(testValidMAC))
			})
			It("env MAC option should work", func() {
				cmdArgs.Args = fmt.Sprintf("MAC=%s", testValidMAC2)
				_ = plugin.CmdAdd(cmdArgs)
				Expect(updatedPluginConf.MAC).To(BeIdenticalTo(testValidMAC2))
			})
			It("runtimeConfig MAC option should work", func() {
				pluginConf.RuntimeConfig.Mac = testValidMAC3
				_ = plugin.CmdAdd(cmdArgs)
				Expect(updatedPluginConf.MAC).To(BeIdenticalTo(testValidMAC3))
			})
			It("runtimeConfig MAC option has higher priority", func() {
				pluginConf.MAC = testValidMAC
				cmdArgs.Args = fmt.Sprintf("MAC=%s", testValidMAC2)
				pluginConf.RuntimeConfig.Mac = testValidMAC3
				_ = plugin.CmdAdd(cmdArgs)
				Expect(updatedPluginConf.MAC).To(BeIdenticalTo(testValidMAC3))
			})
		})

		Context("Update DeviceInfo", func() {
			var (
				tmpFile string
			)

			BeforeEach(func() {
				f, err := os.CreateTemp("", "accbr-deviceinfo")
				Expect(err).NotTo(HaveOccurred())
				tmpFile = f.Name()
			})

			AfterEach(func() {
				Expect(os.Remove(tmpFile)).NotTo(HaveOccurred())
			})

			Context("succeed", func() {
				var (
					cmdCtx *cmdContext
				)
				BeforeEach(func() {
					cmdCtx = &cmdContext{pluginConf: &localtypes.PluginConf{}}
					cmdCtx.pluginConf.Representor = "eth3"
					cmdCtx.pluginConf.RuntimeConfig.CNIDeviceInfoFile = tmpFile
				})
				It("unsupported spec version, should update the version", func() {
					before := []byte(`{"version": "1.0.0", "pci": {"pci-address": "0000:d8:00.2"}}`)
					Expect(os.WriteFile(tmpFile, before, 0600)).NotTo(HaveOccurred())
					Expect(plugin.updateDeviceInfo(cmdCtx)).NotTo(HaveOccurred())
					result, err := os.ReadFile(tmpFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(MatchJSON(
						`{"version": "1.1.0", "pci": {"pci-address": "0000:d8:00.2", "representor-device": "eth3"}}`))

				})
				It("merge DeviceInfo", func() {
					Expect(os.WriteFile(tmpFile,
						[]byte(`{"version": "1.1.0", "pci": {"pci-address": "0000:d8:00.2"}}`), 0600)).NotTo(HaveOccurred())
					Expect(plugin.updateDeviceInfo(cmdCtx)).NotTo(HaveOccurred())
					result, err := os.ReadFile(tmpFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(MatchJSON(
						`{"version": "1.1.0", "pci": {"pci-address": "0000:d8:00.2", "representor-device": "eth3"}}`))

				})
				It("overwrite existing representor-device, keep spec version", func() {
					Expect(os.WriteFile(tmpFile,
						[]byte(`{"version": "1.2.0", "pci": {"pci-address": "0000:d8:00.2", "representor-device": "eth4"}}`),
						0600)).NotTo(HaveOccurred())
					Expect(plugin.updateDeviceInfo(cmdCtx)).NotTo(HaveOccurred())
					result, err := os.ReadFile(tmpFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(MatchJSON(
						`{"version": "1.2.0", "pci": {"pci-address": "0000:d8:00.2", "representor-device": "eth3"}}`))
				})
			})

			It("no CNIDeviceInfoFile option", func() {
				cmdCtx := &cmdContext{pluginConf: &localtypes.PluginConf{}}
				Expect(plugin.updateDeviceInfo(cmdCtx)).NotTo(HaveOccurred())
			})

			It("CNIDeviceInfoFile not exist", func() {
				cmdCtx := &cmdContext{pluginConf: &localtypes.PluginConf{}}
				cmdCtx.pluginConf.RuntimeConfig.CNIDeviceInfoFile = "this-file-doesnt-exist-abcd"
				Expect(plugin.updateDeviceInfo(cmdCtx)).To(HaveOccurred())
			})

			It("Unexpected DeviceInfo file format", func() {
				cmdCtx := &cmdContext{pluginConf: &localtypes.PluginConf{}}
				cmdCtx.pluginConf.RuntimeConfig.CNIDeviceInfoFile = tmpFile

				Expect(os.WriteFile(tmpFile, []byte(`[]`), 0600)).NotTo(HaveOccurred())
				Expect(plugin.updateDeviceInfo(cmdCtx)).To(HaveOccurred())

				Expect(os.WriteFile(tmpFile, []byte(`{"pci": []}`), 0600)).NotTo(HaveOccurred())
				Expect(plugin.updateDeviceInfo(cmdCtx)).To(HaveOccurred())
			})

		})
	})
	Describe("CmdDel", func() {

		successfullyLoadConfig := func() {
			configMock.On("LoadConf", cmdArgs.StdinData, mock.Anything).Run(func(args mock.Arguments) {
				*args[1].(*localtypes.NetConf) = pluginConf.NetConf
			}).Return(nil).Once()
		}

		successfullyLoadCache := func() {
			successfullyLoadConfig()
			cacheMock.On("GetStateRef", pluginConf.Name, cmdArgs.ContainerID, cmdArgs.IfName).
				Return(testValidCacheRef).Once()
			cacheMock.On("Load", testValidCacheRef, mock.Anything).Run(func(args mock.Arguments) {
				*args[1].(*localtypes.PluginConf) = *pluginConf
			}).Return(nil).Once()
		}

		successfullyDetachRepresentor := func() {
			successfullyLoadCache()
			managerMock.On("DetachRepresentor", pluginConf).Return(nil).Once()
		}

		successfullyExecDel := func() {
			successfullyDetachRepresentor()
			ipamMock.On("ExecDel", pluginConf.IPAM.Type, cmdArgs.StdinData).Return(nil).Once()
		}

		successfullyGetNS := func() {
			successfullyExecDel()
			nsMock.On("GetNS", cmdArgs.Netns).Return(netNSMock, nil)
		}

		successfullyReleaseVF := func() {
			successfullyGetNS()
			managerMock.On("ReleaseVF", pluginConf, cmdArgs.IfName, cmdArgs.ContainerID, netNSMock).
				Return(nil)
		}

		successfullyResetVFConfig := func() {
			successfullyReleaseVF()
			managerMock.On("ResetVFConfig", pluginConf).
				Return(nil)
		}

		cleanupClose := func() {
			netNSMock.On("Close").Return(nil)
		}

		cleanupCacheDelete := func() {
			cleanupClose()
			cacheMock.On("Delete", testValidCacheRef).Return(nil)
		}

		Context("Failed scenarios", func() {
			It("Failed to load config", func() {
				configMock.On("LoadConf", cmdArgs.StdinData, mock.Anything).
					Return(errTest)
				Expect(plugin.CmdDel(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to load cache", func() {
				successfullyLoadConfig()
				cacheMock.On("GetStateRef", pluginConf.Name, cmdArgs.ContainerID, cmdArgs.IfName).
					Return(testValidCacheRef).Once()
				cacheMock.On("Load", testValidCacheRef, mock.Anything).
					Return(errTest).Once()
				Expect(plugin.CmdDel(cmdArgs)).ToNot(HaveOccurred())
			})
			It("Failed to call IPAM del", func() {
				successfullyDetachRepresentor()
				ipamMock.On("ExecDel", pluginConf.IPAM.Type, cmdArgs.StdinData).
					Return(errTest).Once()
				Expect(plugin.CmdDel(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to ReleaseVF", func() {
				successfullyGetNS()
				managerMock.On("ReleaseVF", pluginConf, cmdArgs.IfName, cmdArgs.ContainerID, netNSMock).
					Return(errTest)
				cleanupClose()
				Expect(plugin.CmdDel(cmdArgs)).To(HaveOccurred())
			})
			It("Failed to ResetVFConfig", func() {
				successfullyReleaseVF()
				managerMock.On("ResetVFConfig", pluginConf).
					Return(errTest)
				cleanupClose()
				Expect(plugin.CmdDel(cmdArgs)).To(HaveOccurred())
			})
		})
		Context("Successful scenarios", func() {
			It("no NetNs provided", func() {
				cmdArgs.Netns = ""
				Expect(plugin.CmdDel(cmdArgs)).NotTo(HaveOccurred())
			})
			It("Failed to get NS, should return no error", func() {
				successfullyExecDel()
				nsMock.On("GetNS", cmdArgs.Netns).Return(nil, ns.NSPathNotExistErr{}).Once()
				Expect(plugin.CmdDel(cmdArgs)).NotTo(HaveOccurred())
			})
			It("success", func() {
				successfullyResetVFConfig()
				cleanupCacheDelete()
				Expect(plugin.CmdDel(cmdArgs)).NotTo(HaveOccurred())
			})
		})
	})
	Describe("CmdCheck", func() {
		Context("Successful scenarios", func() {
			It("should not fail", func() {
				Expect(plugin.CmdCheck(cmdArgs)).NotTo(HaveOccurred())
			})
		})
	})
})

var _ = Describe("Plugin - test plugin initialization", func() {
	It("Initialize plugin", func() {
		p := NewPlugin()
		Expect(p).NotTo(BeNil())
	})
})
