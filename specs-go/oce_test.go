package specs

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
)

func TestApplyEditsToOCISpec(t *testing.T) {
	testCases := []struct {
		name           string
		config         *spec.Spec
		edits          *ContainerEdits
		expectedResult spec.Spec
		expectedError  bool
	}{
		{
			name:          "nil spec",
			expectedError: true,
		},
		{
			name:   "nil edits",
			config: &spec.Spec{},
			edits:  nil,
		},
		{
			name:   "add env to the empty spec",
			config: &spec.Spec{},
			edits: &ContainerEdits{
				Env: []string{"BAR=BARVALUE1"},
			},
			expectedResult: spec.Spec{
				Process: &spec.Process{
					Env: []string{"BAR=BARVALUE1"},
				},
			},
		},
		{
			name:   "add devices nodes to the empty spec",
			config: &spec.Spec{},
			edits: &ContainerEdits{
				DevicesNodes: []*DeviceNode{
					{
						Path: "/dev/vendorct1",
					},
				},
			},
			expectedResult: spec.Spec{
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{Path: "/dev/vendorct1"},
					},
				},
			},
		},
		{
			name:   "add mounts to the empty spec",
			config: &spec.Spec{},
			edits: &ContainerEdits{
				Mounts: []*Mount{
					{
						HostPath:      "/dev/vendorct1",
						ContainerPath: "/dev/vendorct1",
					},
				},
			},
			expectedResult: spec.Spec{
				Mounts: []spec.Mount{
					{
						Source:      "/dev/vendorct1",
						Destination: "/dev/vendorct1",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ApplyEditsToOCISpec(tc.config, tc.edits)
			if tc.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.edits != nil {
				require.Equal(t, tc.expectedResult, *tc.config)
			}
		})
	}

}
