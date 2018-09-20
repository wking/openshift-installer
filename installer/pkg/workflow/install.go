package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/installer/installer/pkg/config-generator"
	"github.com/openshift/installer/pkg/terraform"
)

// InstallWorkflow creates new instances of the 'install' workflow,
// responsible for running the actions necessary to install a new cluster.
func InstallWorkflow(clusterDir string) Workflow {
	return Workflow{
		metadata: metadata{clusterDir: clusterDir},
		steps: []step{
			readClusterConfigStep,
			generateTerraformVariablesStep,
			generateTLSConfigStep,
			generateClusterConfigMaps,
			generateIgnConfigStep,
			installAssetsStep,
			installInfraStep,
			installBootstrapStep,
		},
	}
}

func installAssetsStep(m *metadata) error {
	return runInstallStep(m, terraform.AssetsStep)
}

func installInfraStep(m *metadata) error {
	return runInstallStep(m, terraform.InfraStep)
}

func installBootstrapStep(m *metadata) error {
	if !terraform.HasStateFile(m.clusterDir, terraform.BootstrapStep) {
		return runInstallStep(m, terraform.BootstrapStep)
	}
	return nil
}

func runInstallStep(m *metadata, step string, extraArgs ...string) error {
	dir, err := baseLocation()
	if err != nil {
		return err
	}
	templateDir, err := terraform.FindStepTemplates(dir, step, m.cluster.Platform)
	if err != nil {
		return err
	}
	if err := terraform.Init(m.clusterDir, templateDir); err != nil {
		return err
	}
	_, err = terraform.Apply(m.clusterDir, step, templateDir, extraArgs...)
	return err
}

func generateIgnConfigStep(m *metadata) error {
	c := configgenerator.New(m.cluster)
	return c.GenerateIgnConfig(m.clusterDir)
}

func generateTLSConfigStep(m *metadata) error {
	if err := os.MkdirAll(filepath.Join(m.clusterDir, tlsPath), os.ModeDir|0755); err != nil {
		return fmt.Errorf("failed to create TLS directory at %s", tlsPath)
	}

	c := configgenerator.New(m.cluster)
	return c.GenerateTLSConfig(m.clusterDir)
}
