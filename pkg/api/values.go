package api

import (
	"os"
)

var (
	FulcioURL     string
	RekorURL      string
	TufURL        string
	OidcIssuerURL string
)

func init() {
	FulcioURL = os.Getenv("FULCIO_URL")
	RekorURL = os.Getenv("REKOR_URL")
	TufURL = os.Getenv("TUF_URL")
	OidcIssuerURL = os.Getenv("OIDC_ISSUER_URL")
}
