package utils

import (
	"errors"

	"github.com/vishvananda/netlink"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/utils/mocks"
)

var (
	errTest1 = errors.New("test")
)

var _ = Describe("Utils", func() {
	Context("Checking GetSriovNumVfs function", func() {
		It("Assuming existing interface", func() {
			result, err := GetSriovNumVfs("enp175s0f1")
			Expect(result).To(Equal(2), "Existing sriov interface should return correct VFs count")
			Expect(err).NotTo(HaveOccurred(), "Existing sriov interface should not return an error")
		})
		It("Assuming not existing interface", func() {
			_, err := GetSriovNumVfs("enp175s0f2")
			Expect(err).To(HaveOccurred(), "Not existing sriov interface should return an error")
		})
	})
	Context("Checking GetVfid function", func() {
		It("Assuming existing interface", func() {
			result, err := GetVfid("0000:af:06.0", "enp175s0f1")
			Expect(result).To(Equal(0), "Existing VF should return correct VF index")
			Expect(err).NotTo(HaveOccurred(), "Existing VF should not return an error")
		})
		It("Assuming not existing interface", func() {
			_, err := GetVfid("0000:af:06.0", "enp175s0f2")
			Expect(err).To(HaveOccurred(), "Not existing interface should return an error")
		})
	})
	Context("Checking HasUserspaceDriver function", func() {
		It("Use userspace driver", func() {
			result, err := HasUserspaceDriver("0000:11:00.0")
			Expect(err).NotTo(HaveOccurred(), "HasUserspaceDriver should not return an error")
			Expect(result).To(BeTrue(), "HasUserspaceDriver should return true")
		})
		It("Has not userspace driver", func() {
			result, err := HasUserspaceDriver("0000:12:00.0")
			Expect(result).To(BeFalse())
			Expect(err).NotTo(HaveOccurred(), "HasUserspaceDriver should not return an error")
		})
	})

	Context("Checking GetParentBridgeForLink function", func() {
		var (
			nLinkMock *mocks.Netlink
		)
		BeforeEach(func() {
			nLinkMock = &mocks.Netlink{}
		})
		AfterEach(func() {
			nLinkMock.AssertExpectations(GinkgoT())
		})
		It("No master", func() {
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{})
			Expect(br).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
		It("Error: Failed to get master for link", func() {
			nLinkMock.On("LinkByIndex", 1).Return(nil, errTest1)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(br).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
		It("Link has unknown master type", func() {
			nLinkMock.On("LinkByIndex", 1).Return(&netlink.Dummy{}, nil)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(br).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
		It("Link is part of a bridge", func() {
			expectedBridgeName := "test1"
			nLinkMock.On("LinkByIndex", 1).Return(
				&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: expectedBridgeName}}, nil)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(err).NotTo(HaveOccurred())
			Expect(br).NotTo(BeNil())
			Expect(br.Attrs().Name).To(BeEquivalentTo(expectedBridgeName))
		})
		It("Link is part of a bond, bond not in a bridge", func() {
			nLinkMock.On("LinkByIndex", 1).Return(
				&netlink.Bond{LinkAttrs: netlink.LinkAttrs{MasterIndex: 0}}, nil)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(err).To(HaveOccurred())
			Expect(br).To(BeNil())
		})
		It("Error: Link is part of a bond, failed to read bond master", func() {
			nLinkMock.On("LinkByIndex", 1).Return(
				&netlink.Bond{LinkAttrs: netlink.LinkAttrs{MasterIndex: 2}}, nil)
			nLinkMock.On("LinkByIndex", 2).Return(nil, errTest1)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(err).To(HaveOccurred())
			Expect(br).To(BeNil())
		})
		It("Link is part of a bond, bond master is not a bridge", func() {
			nLinkMock.On("LinkByIndex", 1).Return(
				&netlink.Bond{LinkAttrs: netlink.LinkAttrs{MasterIndex: 2}}, nil)
			nLinkMock.On("LinkByIndex", 2).Return(&netlink.Dummy{}, nil)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(err).To(HaveOccurred())
			Expect(br).To(BeNil())
		})
		It("Link is part of a bond, bond master is a bridge", func() {
			expectedBridgeName := "test1"
			nLinkMock.On("LinkByIndex", 1).Return(
				&netlink.Bond{LinkAttrs: netlink.LinkAttrs{MasterIndex: 2}}, nil)
			nLinkMock.On("LinkByIndex", 2).Return(
				&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: expectedBridgeName}}, nil)
			br, err := GetParentBridgeForLink(nLinkMock, &netlink.Device{LinkAttrs: netlink.LinkAttrs{MasterIndex: 1}})
			Expect(err).NotTo(HaveOccurred())
			Expect(br).NotTo(BeNil())
			Expect(br.Attrs().Name).To(BeEquivalentTo(expectedBridgeName))
		})
	})
})
