// Code generated by mockery v2.10.0. DO NOT EDIT.

package mocks

import (
	ns "github.com/containernetworking/plugins/pkg/ns"
	types "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
	mock "github.com/stretchr/testify/mock"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// ApplyVFConfig provides a mock function with given fields: conf
func (_m *Manager) ApplyVFConfig(conf *types.PluginConf) error {
	ret := _m.Called(conf)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.PluginConf) error); ok {
		r0 = rf(conf)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AttachRepresentor provides a mock function with given fields: conf
func (_m *Manager) AttachRepresentor(conf *types.PluginConf) error {
	ret := _m.Called(conf)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.PluginConf) error); ok {
		r0 = rf(conf)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DetachRepresentor provides a mock function with given fields: conf
func (_m *Manager) DetachRepresentor(conf *types.PluginConf) error {
	ret := _m.Called(conf)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.PluginConf) error); ok {
		r0 = rf(conf)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ReleaseVF provides a mock function with given fields: conf, podifName, cid, netns
func (_m *Manager) ReleaseVF(conf *types.PluginConf, podifName string, cid string, netns ns.NetNS) error {
	ret := _m.Called(conf, podifName, cid, netns)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.PluginConf, string, string, ns.NetNS) error); ok {
		r0 = rf(conf, podifName, cid, netns)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ResetVFConfig provides a mock function with given fields: conf
func (_m *Manager) ResetVFConfig(conf *types.PluginConf) error {
	ret := _m.Called(conf)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.PluginConf) error); ok {
		r0 = rf(conf)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetupVF provides a mock function with given fields: conf, podifName, cid, netns
func (_m *Manager) SetupVF(conf *types.PluginConf, podifName string, cid string, netns ns.NetNS) (string, error) {
	ret := _m.Called(conf, podifName, cid, netns)

	var r0 string
	if rf, ok := ret.Get(0).(func(*types.PluginConf, string, string, ns.NetNS) string); ok {
		r0 = rf(conf, podifName, cid, netns)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*types.PluginConf, string, string, ns.NetNS) error); ok {
		r1 = rf(conf, podifName, cid, netns)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
