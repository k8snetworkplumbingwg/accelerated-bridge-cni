package plugin

import "github.com/containernetworking/cni/pkg/types"

type envArgs struct {
	types.CommonArgs
	MAC types.UnmarshallableString `json:"mac,omitempty"`
}

func getEnvArgs(envArgsString string) (*envArgs, error) {
	if envArgsString != "" {
		e := envArgs{}
		err := types.LoadArgs(envArgsString, &e)
		if err != nil {
			return nil, err
		}
		return &e, nil
	}
	return nil, nil
}
