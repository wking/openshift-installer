package agent

import (
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/mock"
	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"

	"github.com/openshift/installer/pkg/types/baremetal"
	"github.com/openshift/installer/pkg/types/none"
	"github.com/openshift/installer/pkg/types/vsphere"
)

func TestInstallConfigLoad(t *testing.T) {
	cases := []struct {
		name           string
		data           string
		fetchError     error
		expectedFound  bool
		expectedError  string
		expectedConfig *types.InstallConfig
	}{
		{
			name: "unsupported platform",
			data: `
apiVersion: v1
metadata:
    name: test-cluster
baseDomain: test-domain
platform:
  aws:
    region: us-east-1
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: `invalid install-config configuration: Platform: Unsupported value: "aws": supported values: "baremetal", "vsphere", "none"`,
		},
		{
			name: "apiVips not set for baremetal Compact platform",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14 
    hostPrefix: 23 
  networkType: OpenShiftSDN
  machineNetwork:
  - cidr: 192.168.122.0/23
  serviceNetwork: 
  - 172.30.0.0/16
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 0
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
platform:
  baremetal:
    externalMACAddress: "52:54:00:f6:b4:02"
    provisioningMACAddress: "52:54:00:6e:3b:02"
    ingressVIPs: 
      - 192.168.122.11
    hosts:
      - name: host1
        bootMACAddress: 52:54:01:aa:aa:a1
      - name: host2
        bootMACAddress: 52:54:01:bb:bb:b1
      - name: host3
        bootMACAddress: 52:54:01:cc:cc:c1
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: "failed to create install config: invalid \"install-config.yaml\" file: [platform.baremetal.apiVIPs: Required value: must specify at least one VIP for the API, platform.baremetal.apiVIPs: Required value: must specify VIP for API, when VIP for ingress is set]",
		},
		{
			name: "Required values not set for vsphere platform",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
platform:
  vsphere:
    apiVips:
      - 192.168.122.10
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: "failed to create install config: invalid \"install-config.yaml\" file: [platform.vsphere.apiVIPs: Invalid value: \"192.168.122.10\": IP expected to be in one of the machine networks: 10.0.0.0/16, platform.vsphere.ingressVIPs: Required value: must specify VIP for ingress, when VIP for API is set, platform.vsphere.vCenter: Required value: must specify the name of the vCenter, platform.vsphere.username: Required value: must specify the username, platform.vsphere.password: Required value: must specify the password, platform.vsphere.datacenter: Required value: must specify the datacenter, platform.vsphere.defaultDatastore: Required value: must specify the default datastore]",
		},
		{
			name: "invalid configuration for none platform for sno",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  networkType: OpenShiftNetworking
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 2
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
platform:
  none : {}
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: "invalid install-config configuration: [ControlPlane.Replicas: Required value: ControlPlane.Replicas must be 1 for none platform. Found 3, Compute.Replicas: Required value: Total number of Compute.Replicas must be 0 for none platform. Found 2]",
		},
		{
			name: "no compute.replicas set for SNO",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  networkType: OpenShiftNetworking
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 1
platform:
  none : {}
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: "invalid install-config configuration: Compute.Replicas: Required value: Total number of Compute.Replicas must be 0 for none platform. Found 3",
		},
		{
			name: "invalid networkType for SNO cluster",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  networkType: OpenShiftSDN
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 0
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 1
platform:
  none : {}
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: "invalid install-config configuration: Networking.NetworkType: Invalid value: \"OpenShiftSDN\": Only OpenShiftNetworking network type is allowed for Single Node OpenShift (SNO) cluster",
		},
		{
			name: "invalid platform for SNO cluster",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  networkType: OpenShiftSDN
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 0
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 1
platform:
  aws:
    region: us-east-1
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: false,
			expectedError: "invalid install-config configuration: [Platform: Unsupported value: \"aws\": supported values: \"baremetal\", \"vsphere\", \"none\", Platform: Invalid value: \"aws\": Platform should be set to none if the ControlPlane.Replicas is 1 and total number of Compute.Replicas is 0]",
		},
		{
			name: "valid configuration for none platform for sno",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  networkType: OpenShiftNetworking
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 0
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 1
platform:
  none : {}
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: true,
			expectedConfig: &types.InstallConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: types.InstallConfigVersion,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				AdditionalTrustBundlePolicy: types.PolicyProxyOnly,
				BaseDomain:                  "test-domain",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("10.0.0.0/16")},
					},
					NetworkType:    "OpenShiftNetworking",
					ServiceNetwork: []ipnet.IPNet{*ipnet.MustParseCIDR("172.30.0.0/16")},
					ClusterNetwork: []types.ClusterNetworkEntry{
						{
							CIDR:       *ipnet.MustParseCIDR("10.128.0.0/14"),
							HostPrefix: 23,
						},
					},
				},
				ControlPlane: &types.MachinePool{
					Name:           "master",
					Replicas:       pointer.Int64Ptr(1),
					Hyperthreading: types.HyperthreadingEnabled,
					Architecture:   types.ArchitectureAMD64,
				},
				Compute: []types.MachinePool{
					{
						Name:           "worker",
						Replicas:       pointer.Int64Ptr(0),
						Hyperthreading: types.HyperthreadingEnabled,
						Architecture:   types.ArchitectureAMD64,
					},
				},
				Platform:   types.Platform{None: &none.Platform{}},
				PullSecret: `{"auths":{"example.com":{"auth":"authorization value"}}}`,
				Publish:    types.ExternalPublishingStrategy,
			},
		},
		{
			name: "valid configuration for baremetal platform for HA cluster - deprecated fields",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14 
    hostPrefix: 23 
  networkType: OpenShiftSDN
  machineNetwork:
  - cidr: 192.168.122.0/23
  serviceNetwork: 
  - 172.30.0.0/16
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 2
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
platform:
  baremetal:
    externalMACAddress: "52:54:00:f6:b4:02"
    provisioningMACAddress: "52:54:00:6e:3b:02"
    apiVIP: 192.168.122.10
    ingressVIP: 192.168.122.11
    hosts:
      - name: host1
        bootMACAddress: 52:54:01:aa:aa:a1
      - name: host2
        bootMACAddress: 52:54:01:bb:bb:b1
      - name: host3
        bootMACAddress: 52:54:01:cc:cc:c1
      - name: host4
        bootMACAddress: 52:54:01:dd:dd:d1
      - name: host5
        bootMACAddress: 52:54:01:ee:ee:e1
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: true,
			expectedConfig: &types.InstallConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: types.InstallConfigVersion,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				AdditionalTrustBundlePolicy: types.PolicyProxyOnly,
				BaseDomain:                  "test-domain",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("192.168.122.0/23")},
					},
					NetworkType:    "OpenShiftSDN",
					ServiceNetwork: []ipnet.IPNet{*ipnet.MustParseCIDR("172.30.0.0/16")},
					ClusterNetwork: []types.ClusterNetworkEntry{
						{
							CIDR:       *ipnet.MustParseCIDR("10.128.0.0/14"),
							HostPrefix: 23,
						},
					},
				},
				ControlPlane: &types.MachinePool{
					Name:           "master",
					Replicas:       pointer.Int64Ptr(3),
					Hyperthreading: types.HyperthreadingEnabled,
					Architecture:   types.ArchitectureAMD64,
				},
				Compute: []types.MachinePool{
					{
						Name:           "worker",
						Replicas:       pointer.Int64Ptr(2),
						Hyperthreading: types.HyperthreadingEnabled,
						Architecture:   types.ArchitectureAMD64,
					},
				},
				Platform: types.Platform{
					BareMetal: &baremetal.Platform{
						LibvirtURI:              "qemu:///system",
						ClusterProvisioningIP:   "172.22.0.3",
						BootstrapProvisioningIP: "172.22.0.2",
						ExternalBridge:          "baremetal",
						ExternalMACAddress:      "52:54:00:f6:b4:02",
						ProvisioningNetwork:     "Managed",
						ProvisioningBridge:      "provisioning",
						ProvisioningMACAddress:  "52:54:00:6e:3b:02",
						ProvisioningDHCPRange:   "172.22.0.10,172.22.0.254",
						ProvisioningNetworkCIDR: &ipnet.IPNet{
							IPNet: net.IPNet{
								IP:   []byte("\xac\x16\x00\x00"),
								Mask: []byte("\xff\xff\xff\x00"),
							},
						},
						Hosts: []*baremetal.Host{
							{
								Name:            "host1",
								BootMACAddress:  "52:54:01:aa:aa:a1",
								BootMode:        "UEFI",
								HardwareProfile: "default",
							},
							{
								Name:            "host2",
								BootMACAddress:  "52:54:01:bb:bb:b1",
								BootMode:        "UEFI",
								HardwareProfile: "default",
							},
							{
								Name:            "host3",
								BootMACAddress:  "52:54:01:cc:cc:c1",
								BootMode:        "UEFI",
								HardwareProfile: "default",
							},
							{
								Name:            "host4",
								BootMACAddress:  "52:54:01:dd:dd:d1",
								BootMode:        "UEFI",
								HardwareProfile: "default",
							},
							{
								Name:            "host5",
								BootMACAddress:  "52:54:01:ee:ee:e1",
								BootMode:        "UEFI",
								HardwareProfile: "default",
							}},
						DeprecatedAPIVIP:     "192.168.122.10",
						APIVIPs:              []string{"192.168.122.10"},
						DeprecatedIngressVIP: "192.168.122.11",
						IngressVIPs:          []string{"192.168.122.11"},
					},
				},
				PullSecret: `{"auths":{"example.com":{"auth":"authorization value"}}}`,
				Publish:    types.ExternalPublishingStrategy,
			},
		},
		{
			name: "valid configuration for vsphere platform for compact cluster - deprecated field apiVip",
			data: `
apiVersion: v1
metadata:
  name: test-cluster
baseDomain: test-domain
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14 
    hostPrefix: 23 
  networkType: OpenShiftSDN
  machineNetwork:
  - cidr: 192.168.122.0/23
  serviceNetwork: 
  - 172.30.0.0/16
compute:
  - architecture: amd64
    hyperthreading: Enabled
    name: worker
    platform: {}
    replicas: 0
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
platform:
  vsphere :
    vcenter: 192.168.122.30
    username: testUsername
    password: testPassword
    datacenter: testDataCenter
    defaultDataStore: testDefaultDataStore
    apiVIP: 192.168.122.10
    ingressVIPs: 
      - 192.168.122.11
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: true,
			expectedConfig: &types.InstallConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: types.InstallConfigVersion,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				AdditionalTrustBundlePolicy: types.PolicyProxyOnly,
				BaseDomain:                  "test-domain",
				Networking: &types.Networking{
					MachineNetwork: []types.MachineNetworkEntry{
						{CIDR: *ipnet.MustParseCIDR("192.168.122.0/23")},
					},
					NetworkType:    "OpenShiftSDN",
					ServiceNetwork: []ipnet.IPNet{*ipnet.MustParseCIDR("172.30.0.0/16")},
					ClusterNetwork: []types.ClusterNetworkEntry{
						{
							CIDR:       *ipnet.MustParseCIDR("10.128.0.0/14"),
							HostPrefix: 23,
						},
					},
				},
				ControlPlane: &types.MachinePool{
					Name:           "master",
					Replicas:       pointer.Int64Ptr(3),
					Hyperthreading: types.HyperthreadingEnabled,
					Architecture:   types.ArchitectureAMD64,
				},
				Compute: []types.MachinePool{
					{
						Name:           "worker",
						Replicas:       pointer.Int64Ptr(0),
						Hyperthreading: types.HyperthreadingEnabled,
						Architecture:   types.ArchitectureAMD64,
					},
				},
				Platform: types.Platform{
					VSphere: &vsphere.Platform{
						VCenter:          "192.168.122.30",
						Username:         "testUsername",
						Password:         "testPassword",
						Datacenter:       "testDataCenter",
						DefaultDatastore: "testDefaultDataStore",
						DeprecatedAPIVIP: "192.168.122.10",
						APIVIPs:          []string{"192.168.122.10"},
						IngressVIPs:      []string{"192.168.122.11"},
					},
				},
				PullSecret: `{"auths":{"example.com":{"auth":"authorization value"}}}`,
				Publish:    types.ExternalPublishingStrategy,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			fileFetcher := mock.NewMockFileFetcher(mockCtrl)
			fileFetcher.EXPECT().FetchByName(installConfigFilename).
				Return(
					&asset.File{
						Filename: installConfigFilename,
						Data:     []byte(tc.data)},
					tc.fetchError,
				).MaxTimes(2)

			asset := &OptionalInstallConfig{}
			found, err := asset.Load(fileFetcher)
			assert.Equal(t, tc.expectedFound, found, "unexpected found value returned from Load")
			if tc.expectedError != "" {
				assert.Equal(t, tc.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
			}
			if tc.expectedFound {
				assert.Equal(t, tc.expectedConfig, asset.Config, "unexpected Config in InstallConfig")
			}
		})
	}
}
