package api

import "github.com/spf13/viper"

const (
	FulcioURL      = "SIGSTORE_FULCIO_URL"
	RekorURL       = "SIGSTORE_REKOR_URL"
	TufURL         = "TUF_URL"
	OidcIssuerURL  = "SIGSTORE_OIDC_ISSUER"
	OidcRealm      = "KEYCLOAK_REALM"
	GithubToken    = "TEST_GITHUB_TOKEN" // #nosec G101: Potential hardcoded credentials (gosec)
	GithubUsername = "TEST_GITHUB_USER"
	GithubOwner    = "TEST_GITHUB_OWNER"
	GithubRepo     = "TEST_GITHUB_REPO"
	CliStrategy    = "CLI_STRATEGY"

	// 'DockerRegistry*' - Login credentials for 'registry.redhat.io'.
	DockerRegistryUsername = "REGISTRY_USERNAME"
	DockerRegistryPassword = "REGISTRY_PASSWORD"
)

var Values *viper.Viper

func init() {
	Values = viper.New()

	Values.SetDefault(OidcRealm, "trusted-artifact-signer")
	Values.SetDefault(GithubUsername, "ignore")
	Values.SetDefault(GithubOwner, "securesign")
	Values.SetDefault(GithubRepo, "e2e-gitsign-test")
	Values.SetDefault(CliStrategy, "local")
	Values.AutomaticEnv()
}

func GetValueFor(key string) string {
	return Values.GetString(key)
}
