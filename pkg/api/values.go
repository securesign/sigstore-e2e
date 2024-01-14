package api

import "github.com/spf13/viper"

const (
	FulcioURL         = "FULCIO_URL"
	RekorURL          = "REKOR_URL"
	TufURL            = "TUF_URL"
	OidcIssuerURL     = "OIDC_ISSUER_URL"
	OidcRealm         = "KEYCLOAK_REALM"
	KeycloakURL       = "KEYCLOAK_URL"
	GithubToken       = "TEST_GITHUB_TOKEN" // #nosec G101: Potential hardcoded credentials (gosec)
	GithubUsername    = "TEST_GITHUB_USER"
	GithubOwner       = "TEST_GITHUB_OWNER"
	GithubRepo        = "TEST_GITHUB_REPO"
	CosignRepo        = "COSIGN_GITHUB_REPO"
	CosignRepoBranch  = "COSIGN_GITHUB_REPO_BRANCH"
	GitsignRepo       = "GITSIGN_GITHUB_REPO"
	GitsignRepoBranch = "GITSIGN_GITHUB_REPO_BRANCH"

	// 'DockerRegistry*' - Login credentials for 'registry.redhat.io'.
	DockerRegistryUsername = "REGISTRY_USERNAME"
	DockerRegistryPassword = "REGISTRY_PASSWORD"
)

var Values *viper.Viper

func init() {
	Values = viper.New()

	Values.SetDefault(OidcRealm, "sigstore")
	Values.SetDefault(GithubUsername, "ignore")
	Values.SetDefault(GithubOwner, "securesign")
	Values.SetDefault(GithubRepo, "e2e-gitsign-test")
	Values.SetDefault(CosignRepo, "https://github.com/sigstore/cosign")
	Values.SetDefault(CosignRepoBranch, "main")
	Values.SetDefault(GitsignRepo, "https://github.com/sigstore/gitsign")
	Values.SetDefault(GitsignRepoBranch, "main")
	Values.AutomaticEnv()
}

func GetValueFor(key string) string {
	return Values.GetString(key)
}
