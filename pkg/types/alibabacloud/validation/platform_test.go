package validation

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/alibabacloud"
)

func validPlatform() *alibabacloud.Platform {
	return &alibabacloud.Platform{
		Region: "cn-hangzhou",
	}
}

func validNetworking() *types.Networking {
	return &types.Networking{
		NetworkType: "OpenShiftNetworking",
		MachineNetwork: []types.MachineNetworkEntry{{
			CIDR: *ipnet.MustParseCIDR("10.0.0.0/16"),
		}},
	}
}

func invalidMachineNetwork() *types.Networking {
	return &types.Networking{
		NetworkType: "OpenShiftNetworking",
		MachineNetwork: []types.MachineNetworkEntry{{
			CIDR: *ipnet.MustParseCIDR("100.100.100.0/24"),
		}},
	}
}

func invalidServiceNetwork() *types.Networking {
	return &types.Networking{
		NetworkType: "OpenShiftNetworking",
		ServiceNetwork: []ipnet.IPNet{{
			IPNet: net.IPNet{
				IP:   net.IP{0x64, 0x64, 0x64, 0x00},
				Mask: net.IPMask{0xff, 0xff, 0xff, 0x00},
			},
		}},
	}
}

func invalidClusterNetwork() *types.Networking {
	return &types.Networking{
		NetworkType: "OpenShiftNetworking",
		ClusterNetwork: []types.ClusterNetworkEntry{{
			CIDR: ipnet.IPNet{
				IPNet: net.IPNet{
					IP:   net.IP{0x64, 0x64, 0x64, 0x00},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0x00},
				},
			},
		}},
	}
}

func TestValidatePlatform(t *testing.T) {
	cases := []struct {
		name       string
		platform   *alibabacloud.Platform
		networking *types.Networking
		expected   string
	}{
		{
			name:       "minimal",
			platform:   validPlatform(),
			networking: validNetworking(),
		},
		{
			name: "invalid region",
			platform: &alibabacloud.Platform{
				Region:          "",
				ResourceGroupID: "test-resource-group",
			},
			expected:   `^test-path\.region: Required value: region must be specified$`,
			networking: validNetworking(),
		},
		{
			name: "invalid vpc ID for existing VSwitches",
			platform: &alibabacloud.Platform{
				Region:     "cn-hangzhou",
				VpcID:      "",
				VSwitchIDs: []string{"vsw-test"},
			},
			expected:   `^test-path\.vpcID: Required value: when using existing VSwitches, an existing VPC must be used$`,
			networking: validNetworking(),
		},
		{
			name: "duplicate VSwitch ID",
			platform: &alibabacloud.Platform{
				Region:     "cn-hangzhou",
				VpcID:      "vpc-test",
				VSwitchIDs: []string{"vsw-test", "vsw-test"},
			},
			expected:   `^test-path\.vswitchIDs\[1\]: Duplicate value: \"vsw-test\"$`,
			networking: validNetworking(),
		},
		{
			name: "invalid vpc ID for existing private zones",
			platform: &alibabacloud.Platform{
				Region:        "cn-hangzhou",
				VpcID:         "",
				PrivateZoneID: "pvtz-test",
			},
			expected:   `^test-path\.vpcID: Required value: when using existing privatezones, an existing VPC must be used$`,
			networking: validNetworking(),
		},
		{
			name: "valid machine pool",
			platform: &alibabacloud.Platform{
				Region:                 "cn-hangzhou",
				ResourceGroupID:        "test-resource-group",
				DefaultMachinePlatform: &alibabacloud.MachinePool{},
			},
			networking: validNetworking(),
		},
		{
			name:       "invalid machine network",
			platform:   validPlatform(),
			expected:   `^networking\.machineNetwork: Invalid value: "100\.100\.100\.0/24": contains 100\.100\.100\.200 which is reserved for the metadata service$`,
			networking: invalidMachineNetwork(),
		},
		{
			name:       "invalid service network",
			platform:   validPlatform(),
			expected:   `^networking\.serviceNetwork: Invalid value: "100\.100\.100\.0/24": contains 100\.100\.100\.200 which is reserved for the metadata service$`,
			networking: invalidServiceNetwork(),
		},
		{
			name:       "invalid cluster network",
			platform:   validPlatform(),
			expected:   `^networking\.clusterNetwork: Invalid value: "100\.100\.100\.0/24": contains 100\.100\.100\.200 which is reserved for the metadata service$`,
			networking: invalidClusterNetwork(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePlatform(tc.platform, tc.networking, field.NewPath("test-path")).ToAggregate()
			if tc.expected == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexp(t, tc.expected, err)
			}
		})
	}
}
