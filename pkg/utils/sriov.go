package utils

import "github.com/Mellanox/sriovnet"

// SriovnetProvider represents limited subset of functions from sriovnet package
type SriovnetProvider interface {
	GetVfRepresentor(string, int) (string, error)
	GetUplinkRepresentor(string) (string, error)
}

type SriovnetWrapper struct{}

func (s *SriovnetWrapper) GetVfRepresentor(master string, vfid int) (string, error) {
	return sriovnet.GetVfRepresentor(master, vfid)
}

func (s *SriovnetWrapper) GetUplinkRepresentor(vfPciAddress string) (string, error) {
	return sriovnet.GetUplinkRepresentor(vfPciAddress)
}
