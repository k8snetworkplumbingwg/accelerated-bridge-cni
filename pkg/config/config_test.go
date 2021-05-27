package config

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Checking LoadConf function", func() {
		It("Assuming correct config file - existing DeviceID", func() {
			conf := []byte(`{
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
			_, err := LoadConf(conf)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming incorrect config file - not existing DeviceID", func() {
			conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.3",
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
			_, err := LoadConf(conf)
			Expect(err).To(HaveOccurred())
		})
		It("Assuming incorrect config file - broken json", func() {
			conf := []byte(`{
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
			_, err := LoadConf(conf)
			Expect(err).To(HaveOccurred())
		})
	})
	It("Assuming correct config file - complex trunk config", func() {
		conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.1",
		"trunk" : [{ "id" : 5 }, { "id": 19, "minID" : 101, "maxID" : 103 }, {"id": 55}, { "minID" : 20, "maxID" : 23 }]
		}`)
		cfg, err := LoadConf(conf)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Trunk).To(BeEquivalentTo([]int{5, 19, 20, 21, 22, 23, 55, 101, 102, 103}))

	})
	It("Assuming incorrect config file - negative vlan ID", func() {
		conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.1",
		"vlan" : -222
		}`)
		_, err := LoadConf(conf)
		Expect(err).To(HaveOccurred())
	})
	It("Assuming incorrect config file - vlan ID to large", func() {
		conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.1",
		"vlan" : 4095
		}`)
		_, err := LoadConf(conf)
		Expect(err).To(HaveOccurred())
	})
	It("Assuming incorrect config file - trunk minID more that maxID", func() {
		conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.1",
		"trunk" : [ { "minID" : 1000, "maxID" : 50 } ]
		}`)
		_, err := LoadConf(conf)
		Expect(err).To(HaveOccurred())
	})
	It("Assuming incorrect config file - trunk negative id", func() {
		conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.1",
		"trunk" : [ { "id" : -1000 } ]
		}`)
		_, err := LoadConf(conf)
		Expect(err).To(HaveOccurred())
	})
	It("Assuming incorrect config file - trunk invalid range", func() {
		conf := []byte(`{
		"name": "mynet",
		"type": "accelerated-bridge",
		"deviceID": "0000:af:06.1",
		"trunk" : [ { "minID" : 4000, "maxID": 5000 } ]
		}`)
		_, err := LoadConf(conf)
		Expect(err).To(HaveOccurred())
	})
	Context("Checking getVfInfo function", func() {
		It("Assuming existing PF", func() {
			_, _, err := getVfInfo("0000:af:06.0")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming not existing PF", func() {
			_, _, err := getVfInfo("0000:af:07.0")
			Expect(err).To(HaveOccurred())
		})
	})
})
