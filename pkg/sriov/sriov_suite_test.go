package sriov

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/DmytroLinkin/switchdev-cni/pkg/utils"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}
func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sriov Suite")
}

var _ = BeforeSuite(func() {
	// create test sys tree
	err := utils.CreateTmpSysFs()
	check(err)
})

var _ = AfterSuite(func() {
	var err error
	err = utils.RemoveTmpSysFs()
	check(err)
})
