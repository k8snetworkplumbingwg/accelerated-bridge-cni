package manager

import (
	"github.com/Mellanox/sriovnet"
)

// Sriovnet represents limited subset of functions from sriovnet package
type Sriovnet interface {
	GetVfRepresentor(string, int) (string, error)
}

type sriovnetWrapper struct{}

func (s *sriovnetWrapper) GetVfRepresentor(master string, vfid int) (string, error) {
	return sriovnet.GetVfRepresentor(master, vfid)
}
