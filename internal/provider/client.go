package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

// vaultClient wraps the Vault/OpenBao API client with the operations this
// provider needs to manage the Rancher secrets engine.
type vaultClient struct {
	api *vault.Client
}

func newVaultClient(address, token string) (*vaultClient, error) {
	cfg := vault.DefaultConfig()
	if cfg.Error != nil {
		return nil, cfg.Error
	}
	if address != "" {
		cfg.Address = address
	}
	c, err := vault.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if token != "" {
		c.SetToken(token)
	}
	return &vaultClient{api: c}, nil
}

// mountConfig holds the tunable options applied when enabling the engine.
type mountConfig struct {
	DefaultLeaseTTL string
	MaxLeaseTTL     string
}

// enableMount mounts the secrets engine of the given plugin type at path.
func (c *vaultClient) enableMount(ctx context.Context, path, pluginType, description string, cfg mountConfig) error {
	input := &vault.MountInput{
		Type:        pluginType,
		Description: description,
		Config: vault.MountConfigInput{
			DefaultLeaseTTL: cfg.DefaultLeaseTTL,
			MaxLeaseTTL:     cfg.MaxLeaseTTL,
		},
	}
	return c.api.Sys().MountWithContext(ctx, path, input)
}

// tuneMount updates tunable options of an existing mount (e.g. description).
func (c *vaultClient) tuneMount(ctx context.Context, path, description string, cfg mountConfig) error {
	input := vault.MountConfigInput{
		Description:     &description,
		DefaultLeaseTTL: cfg.DefaultLeaseTTL,
		MaxLeaseTTL:     cfg.MaxLeaseTTL,
	}
	return c.api.Sys().TuneMountWithContext(ctx, path, input)
}

// mountInfo returns the mount at the given path, or nil if it does not exist.
func (c *vaultClient) mountInfo(ctx context.Context, path string) (*vault.MountOutput, error) {
	mounts, err := c.api.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	key := strings.TrimRight(path, "/") + "/"
	if m, ok := mounts[key]; ok {
		return m, nil
	}
	return nil, nil
}

// disableMount unmounts the secrets engine at path.
func (c *vaultClient) disableMount(ctx context.Context, path string) error {
	return c.api.Sys().UnmountWithContext(ctx, path)
}

// write writes data to an arbitrary path under the backend mount.
func (c *vaultClient) write(ctx context.Context, path string, data map[string]interface{}) error {
	_, err := c.api.Logical().WriteWithContext(ctx, path, data)
	return err
}

// read reads an arbitrary path under the backend mount, returning nil if absent.
func (c *vaultClient) read(ctx context.Context, path string) (map[string]interface{}, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, nil
	}
	return secret.Data, nil
}

// delete removes an arbitrary path under the backend mount.
func (c *vaultClient) delete(ctx context.Context, path string) error {
	_, err := c.api.Logical().DeleteWithContext(ctx, path)
	return err
}

func configPath(backend string) string {
	return fmt.Sprintf("%s/config", strings.TrimRight(backend, "/"))
}

func serviceAccountPath(backend, name string) string {
	return fmt.Sprintf("%s/service-accounts/%s", strings.TrimRight(backend, "/"), name)
}

func rolePath(backend, name string) string {
	return fmt.Sprintf("%s/roles/%s", strings.TrimRight(backend, "/"), name)
}

// isMountAlreadyExists reports whether the error is Vault's "path is already in
// use" response, so callers can surface a friendlier message.
func isMountAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		for _, e := range respErr.Errors {
			if strings.Contains(e, "path is already in use") {
				return true
			}
		}
	}
	return false
}
