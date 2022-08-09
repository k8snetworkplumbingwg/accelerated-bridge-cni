package config

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	localtypes "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/utils/mocks"
)

var _ = Describe("Config", func() {
	const existingVfPrefix = "0000:af:06."
	const nonExistentVF = "0000:af:07.0"
	const existingPF = "enp175s0f1"

	var (
		mockSriovnet *mocks.Sriovnet
		mockNetlink  *mocks.Netlink
		conf         Config
		pluginConf   *localtypes.PluginConf
	)

	BeforeEach(func() {
		mockSriovnet = &mocks.Sriovnet{}
		mockNetlink = &mocks.Netlink{}
		pluginConf = &localtypes.PluginConf{}
		conf = Config{sriovnetProvider: mockSriovnet, netlink: mockNetlink}
	})

	AfterEach(func() {
		mockSriovnet.AssertExpectations(GinkgoT())
		mockNetlink.AssertExpectations(GinkgoT())
	})

	Context("Checking ParseConf function", func() {
		When("Broken config format", func() {
			It("Invalid configuration - Broken json", func() {
				data := []byte(`{
						"name": "mynet"
						"type": "accelerated-bridge",
						"deviceID": "0000:af:06.1",
						"vf": 0,
						"ipam": {
							"type": "host-local",
							"subnet": "10.55.206.0/26",
							"routes": [
						{ "dst": "0.0.0.0/0" }
							],
							"gateway": "10.55.206.1"
						}
					}`)
				err := conf.ParseConf(data, pluginConf)
				Expect(err).To(HaveOccurred())
			})
		})
		When("DeviceID exist", func() {
			BeforeEach(func() {
				mockSriovnet.On("GetUplinkRepresentor", mock.MatchedBy(func(pciAddr string) bool {
					return strings.HasPrefix(pciAddr, existingVfPrefix)
				})).Return(existingPF, nil)
			})
			Context("Basic checks", func() {
				It("Valid configuration", func() {
					data := []byte(`{
						"name": "mynet",
						"type": "accelerated-bridge",
						"deviceID": "0000:af:06.1",
						"vf": 0,
						"vlan": 100,
						"trunk" : [ { "id" : 42 }, { "minID" : 1000, "maxID" : 1010 } ],
						"ipam": {
							"type": "host-local",
							"subnet": "10.55.206.0/26",
							"routes": [
						{ "dst": "0.0.0.0/0" }
							],
							"gateway": "10.55.206.1"
						}
					}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).NotTo(HaveOccurred())
				})
			})
			Context("VLAN config checks", func() {
				It("Valid configuration - complex trunk config", func() {
					data := []byte(`{
							"name": "mynet",
							"type": "accelerated-bridge",
							"deviceID": "0000:af:06.1",
							"trunk" : [{ "id" : 5 },
								{ "id": 19, "minID" : 101, "maxID" : 103 },
								{"id": 55}, { "minID" : 20, "maxID" : 23 }]
							}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).NotTo(HaveOccurred())
					Expect(pluginConf.Trunk).To(BeEquivalentTo([]int{5, 19, 20, 21, 22, 23, 55, 101, 102, 103}))
				})
				It("Invalid configuration - negative vlan ID", func() {
					data := []byte(`{
							"name": "mynet",
							"type": "accelerated-bridge",
							"deviceID": "0000:af:06.1",
							"vlan" : -222
							}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).To(HaveOccurred())
				})
				It("Invalid configuration - vlan ID to large", func() {
					data := []byte(`{
							"name": "mynet",
							"type": "accelerated-bridge",
							"deviceID": "0000:af:06.1",
							"vlan" : 4095
							}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).To(HaveOccurred())
				})
				It("Invalid configuration - trunk minID more that maxID", func() {
					data := []byte(`{
							"name": "mynet",
							"type": "accelerated-bridge",
							"deviceID": "0000:af:06.1",
							"trunk" : [ { "minID" : 1000, "maxID" : 50 } ]
							}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).To(HaveOccurred())
				})
				It("Invalid configuration - trunk negative id", func() {
					data := []byte(`{
							"name": "mynet",
							"type": "accelerated-bridge",
							"deviceID": "0000:af:06.1",
							"trunk" : [ { "id" : -1000 } ]
							}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).To(HaveOccurred())
				})
				It("Invalid configuration - trunk invalid range", func() {
					data := []byte(`{
							"name": "mynet",
							"type": "accelerated-bridge",
							"deviceID": "0000:af:06.1",
							"trunk" : [ { "minID" : 0, "maxID": 4095 } ]
							}`)
					err := conf.ParseConf(data, pluginConf)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("Bridge config checks", func() {
				configFmt := `{
								"name": "mynet",
								"type": "accelerated-bridge",
								"deviceID": "0000:af:06.1",
								"bridge": "%s"
							}`
				When("Static bridge configuration", func() {
					It("Valid configuration - use default bridge if not specified", func() {
						data := []byte(`{
								"name": "mynet",
								"type": "accelerated-bridge",
								"deviceID": "0000:af:06.1"
							}`)
						Expect(conf.ParseConf(data, pluginConf)).NotTo(HaveOccurred())
						Expect(pluginConf.ActualBridge).To(Equal(DefaultBridge))
					})
					It("Invalid config - invalid format for bridge option", func() {
						Expect(conf.ParseConf(
							[]byte(fmt.Sprintf(configFmt, ", ")),
							pluginConf)).To(HaveOccurred())
						Expect(conf.ParseConf(
							[]byte(fmt.Sprintf(configFmt, "foo,,")),
							pluginConf)).To(HaveOccurred())
						Expect(conf.ParseConf(
							[]byte(fmt.Sprintf(configFmt, ",foo")),
							pluginConf)).To(HaveOccurred())
					})
				})
				When("Autodetect bridge", func() {
					BeforeEach(func() {
						mockNetlink.On("LinkByName", mock.Anything).Return(
							&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: existingPF, MasterIndex: 2}}, nil)
						mockNetlink.On("LinkByIndex", 2).Return(
							&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "bridge2"}}, nil)
					})
					It("Valid config - autodetect bridge", func() {
						err := conf.ParseConf([]byte(fmt.Sprintf(configFmt, "bridge1, bridge2")), pluginConf)
						Expect(err).NotTo(HaveOccurred())
						Expect(pluginConf.ActualBridge).To(BeEquivalentTo("bridge2"))
					})
					It("Invalid config - uplink attached to unknown bridge", func() {
						err := conf.ParseConf([]byte(fmt.Sprintf(configFmt, "foo, bridge1")), pluginConf)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
		When("DeviceID doesn't exist", func() {
			BeforeEach(func() {
				mockSriovnet.On("GetUplinkRepresentor", nonExistentVF).
					Return("", fmt.Errorf("nonexistent VF"))
			})
			It("DeviceID not found - should return error", func() {
				data := []byte(`{
					"name": "mynet",
					"type": "accelerated-bridge",
					"deviceID": "0000:af:07.0",
					"vf": 0,
					"ipam": {
						"type": "host-local",
						"subnet": "10.55.206.0/26",
						"routes": [
					{ "dst": "0.0.0.0/0" }
						],
						"gateway": "10.55.206.1"
					}
				}`)
				err := conf.ParseConf(data, pluginConf)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Checking getVfInfo function", func() {
		It("Assuming existing PF", func() {
			mockSriovnet.On("GetUplinkRepresentor", mock.MatchedBy(func(pciAddr string) bool {
				return strings.HasPrefix(pciAddr, existingVfPrefix)
			})).Return(existingPF, nil)
			_, _, err := conf.getVfInfo("0000:af:06.0")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming not existing PF", func() {
			mockSriovnet.On("GetUplinkRepresentor", nonExistentVF).
				Return("", fmt.Errorf("nonexistent VF"))
			_, _, err := conf.getVfInfo(nonExistentVF)
			Expect(err).To(HaveOccurred())
		})
	})
})
