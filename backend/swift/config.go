package swift

import (
	"os"
	"strings"

	"github.com/rubiojr/rapi/internal/errors"
	"github.com/rubiojr/rapi/internal/options"
)

// Config contains basic configuration needed to specify swift location for a swift server
type Config struct {
	UserName     string
	Domain       string
	APIKey       string
	AuthURL      string
	Region       string
	Tenant       string
	TenantID     string
	TenantDomain string
	TrustID      string

	StorageURL string
	AuthToken  string

	// auth v3 only
	ApplicationCredentialID     string
	ApplicationCredentialName   string
	ApplicationCredentialSecret string

	Container              string
	Prefix                 string
	DefaultContainerPolicy string

	Connections uint `option:"connections" help:"set a limit for the number of concurrent connections (default: 5)"`
}

func init() {
	options.Register("swift", Config{})
}

// NewConfig returns a new config with the default values filled in.
func NewConfig() Config {
	return Config{
		Connections: 5,
	}
}

// ParseConfig parses the string s and extract swift's container name and prefix.
func ParseConfig(s string) (interface{}, error) {
	data := strings.SplitN(s, ":", 3)
	if len(data) != 3 {
		return nil, errors.New("invalid URL, expected: swift:container-name:/[prefix]")
	}

	scheme, container, prefix := data[0], data[1], data[2]
	if scheme != "swift" {
		return nil, errors.Errorf("unexpected prefix: %s", data[0])
	}

	if len(prefix) == 0 {
		return nil, errors.Errorf("prefix is empty")
	}

	if prefix[0] != '/' {
		return nil, errors.Errorf("prefix does not start with slash (/)")
	}
	prefix = prefix[1:]

	cfg := NewConfig()
	cfg.Container = container
	cfg.Prefix = prefix

	return cfg, nil
}

// ApplyEnvironment saves values from the environment to the config.
func ApplyEnvironment(prefix string, cfg interface{}) error {
	c := cfg.(*Config)
	for _, val := range []struct {
		s   *string
		env string
	}{
		// v2/v3 specific
		{&c.UserName, prefix + "OS_USERNAME"},
		{&c.APIKey, prefix + "OS_PASSWORD"},
		{&c.Region, prefix + "OS_REGION_NAME"},
		{&c.AuthURL, prefix + "OS_AUTH_URL"},

		// v3 specific
		{&c.Domain, prefix + "OS_USER_DOMAIN_NAME"},
		{&c.Tenant, prefix + "OS_PROJECT_NAME"},
		{&c.TenantDomain, prefix + "OS_PROJECT_DOMAIN_NAME"},

		// v2 specific
		{&c.TenantID, prefix + "OS_TENANT_ID"},
		{&c.Tenant, prefix + "OS_TENANT_NAME"},

		// v1 specific
		{&c.AuthURL, prefix + "ST_AUTH"},
		{&c.UserName, prefix + "ST_USER"},
		{&c.APIKey, prefix + "ST_KEY"},

		// Application Credential auth
		{&c.ApplicationCredentialID, prefix + "OS_APPLICATION_CREDENTIAL_ID"},
		{&c.ApplicationCredentialName, prefix + "OS_APPLICATION_CREDENTIAL_NAME"},
		{&c.ApplicationCredentialSecret, prefix + "OS_APPLICATION_CREDENTIAL_SECRET"},

		// Manual authentication
		{&c.StorageURL, prefix + "OS_STORAGE_URL"},
		{&c.AuthToken, prefix + "OS_AUTH_TOKEN"},

		{&c.DefaultContainerPolicy, prefix + "SWIFT_DEFAULT_CONTAINER_POLICY"},
	} {
		if *val.s == "" {
			*val.s = os.Getenv(val.env)
		}
	}
	return nil
}
