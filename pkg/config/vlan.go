// Copyright 2021 NVIDIA CORPORATION
// Copyright 2018-2019 Red Hat, Inc.
// Copyright 2014 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"sort"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
)

func splitVlanIds(trunks []types.Trunk) ([]int, error) {
	vlans := make(map[int]bool)
	for _, item := range trunks {
		var minID, maxID, id int
		if item.MinID != nil {
			minID = *item.MinID
			if vlanIDIsOutOfRange(minID) {
				return nil, errors.New("incorrect trunk minID parameter")
			}
		}
		if item.MaxID != nil {
			maxID = *item.MaxID
			if vlanIDIsOutOfRange(maxID) {
				return nil, errors.New("incorrect trunk maxID parameter")
			}
			if maxID < minID {
				return nil, errors.New("minID is greater than maxID in trunk parameter")
			}
		}
		if minID > 0 && maxID > 0 {
			for v := minID; v <= maxID; v++ {
				vlans[v] = true
			}
		}
		if item.ID != nil {
			id = *item.ID
			if vlanIDIsOutOfRange(id) {
				return nil, errors.New("incorrect trunk id parameter")
			}
			vlans[id] = true
		}
	}
	if len(vlans) == 0 {
		return nil, errors.New("trunk parameter is misconfigured")
	}
	vlanIds := make([]int, 0, len(vlans))
	for k := range vlans {
		vlanIds = append(vlanIds, k)
	}
	sort.Slice(vlanIds, func(i, j int) bool { return vlanIds[i] < vlanIds[j] })
	return vlanIds, nil
}

// check that vlanID is in range 1-4094
// reserved VLANs (0, 4095) can't be set on Linux bridge
func vlanIDIsOutOfRange(vlanID int) bool {
	return vlanID < 1 || vlanID > 4094
}
