package aws

var (
	defaultMachineClass = map[string]string{
		"ap-northeast-1": "m4",
		"ap-northeast-2": "m4",
		"ap-northeast-3": "m4",
		"ap-south-1":     "m4",
		"ap-southeast-1": "m4",
		"ap-southeast-2": "m4",
		"ca-central-1":   "m4",
		"eu-central-1":   "m4",
		"eu-north-1":     "m5",
		"eu-west-1":      "m4",
		"eu-west-2":      "m4",
		"eu-west-3":      "m5",
		"sa-east-1":      "m4",
		"us-east-1":      "m4",
		"us-east-2":      "m4",
		"us-gov-east-1":  "m5",
		"us-gov-west-1":  "m4",
		"us-west-1":      "m4",
		"us-west-2":      "m4",
	}
)

// Platform stores all the global configuration that all machinesets
// use.
type Platform struct {
	// Region specifies the AWS region where the cluster will be created.
	Region string `json:"region"`

	// UserTags specifies additional tags for AWS resources created for the cluster.
	// +optional
	UserTags map[string]string `json:"userTags,omitempty"`

	// DefaultMachinePlatform is the default configuration used when
	// installing on AWS for machine pools which do not define their own
	// platform configuration.
	// +optional
	DefaultMachinePlatform *MachinePool `json:"defaultMachinePlatform,omitempty"`
}

func (p Platform) GetDefaultInstanceClass() string {
	region := p.Region
	if class, ok := defaultMachineClass[region]; ok {
		return class
	}
	return "m4"
}
