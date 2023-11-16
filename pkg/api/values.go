package api

import "github.com/spf13/viper"

const (
	FulcioURL     = "FULCIO_URL"
	RekorURL      = "REKOR_URL"
	TufURL        = "TUF_URL"
	OidcIssuerURL = "OIDC_ISSUER_URL"
	OidcRealm     = "KEYCLOAK_REALM"
	KeycloakUrl   = "KEYCLOAK_URL"
)

var Values *viper.Viper

func init() {
	Values = viper.New()

	viper.SetDefault(OidcRealm, "sigstore")
	Values.AutomaticEnv()
}
