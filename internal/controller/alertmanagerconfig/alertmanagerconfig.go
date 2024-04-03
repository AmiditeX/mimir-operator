package alertmanagerconfig

import (
	"context"
	domain "mimir-operator/api/v1alpha1"
	"mimir-operator/internal/mimirtool"
	"os"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// deleteAlertManagerConfigForTenant deletes the alert manager configuration from Mimir for a specific tenant
func (r *AlertManagerConfigReconciler) deleteAlertManagerConfigForTenant(ctx context.Context, auth *mimirtool.Authentication, mr *domain.AlertManagerConfig) error {
	// Delete the configuration
	err := mimirtool.DeleteAlertManagerConfig(ctx, auth, mr.Spec.ID, mr.Spec.URL)
	if err != nil {
		return err
	}
	return nil
}

// configToString reads an AlertManagerConfig CRD keeps only the Config Spec
// The other fields are irrelevant to Mimir as we only need to apply the config part for the alert manager
func (r *AlertManagerConfigReconciler) configToString(config *domain.AlertManagerConfig) (string, error) {
	// Re-marshal to keep only the ".groups" out of the ".spec"
	result, err := yaml.Marshal(config.Spec.Config)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// sendAMConfigToMimir check if the config is a valid alert manager config
// And then load it with the remote Mimir
func sendAMConfigToMimir(ctx context.Context, auth *mimirtool.Authentication, tenantId, url, config string) error {

	// Put the config on the FS
	configName := "amc_" + tenantId
	fileName, err := dumpConfigToFS(tenantId, configName, config)
	if err != nil {
		return err
	}

	// Cleanup after ourselves
	defer func() {
		if err := os.RemoveAll(fileName); err != nil {
			log.FromContext(ctx).
				WithValues("alertmanagerconfig", tenantId).
				Error(err, "failed to cleanup fs after loading alert manager configuration to mimir")
		}
	}()

	// Verify alert manager configuration before loading it
	err = mimirtool.VerifyAlertManagerConfig(ctx, auth, fileName)

	if err != nil {
		log.FromContext(ctx).
			WithValues("alertmanagerconfig", tenantId).
			Error(err, "failed to validate configuration")
		return err
	}
	err = mimirtool.LoadAlertManagerConfig(ctx, auth, fileName, tenantId, url)

	return err
}

// dumpConfigToFS writes a config for a specific tenant into the filesystem
func dumpConfigToFS(tenant string, configName, config string) (string, error) {
	path := temporaryFiles + tenant + "/"

	_ = os.Mkdir(path, os.ModePerm)

	fileName := path + configName
	return fileName, os.WriteFile(fileName, []byte(config), 0644)
}
