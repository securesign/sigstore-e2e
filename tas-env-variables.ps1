# Get the URLs and export them as environment variables
$TUF_URL = $(oc get tuf -o jsonpath='{.items[0].status.url}' -n trusted-artifact-signer)
$OIDC_ROUTE = $(oc get route keycloak -n keycloak-system --template='{{.spec.host}}')
$OIDC_ISSUER_URL = "https://$OIDC_ROUTE/auth/realms/trusted-artifact-signer"
$COSIGN_FULCIO_URL = $(oc get fulcio -o jsonpath='{.items[0].status.url}' -n trusted-artifact-signer)
$COSIGN_REKOR_URL = $(oc get rekor -o jsonpath='{.items[0].status.url}' -n trusted-artifact-signer)

# Print the URLs
Write-Output "TUF_URL: $TUF_URL"
Write-Output "OIDC_ISSUER_URL: $OIDC_ISSUER_URL"
Write-Output "COSIGN_FULCIO_URL: $COSIGN_FULCIO_URL"
Write-Output "COSIGN_REKOR_URL: $COSIGN_REKOR_URL"

# Export the environment variables for the current session
$env:TUF_URL = $TUF_URL
$env:OIDC_ISSUER_URL = $OIDC_ISSUER_URL
$env:COSIGN_FULCIO_URL = $COSIGN_FULCIO_URL
$env:COSIGN_REKOR_URL = $COSIGN_REKOR_URL

$env:COSIGN_MIRROR = $TUF_URL
$env:COSIGN_ROOT = "$TUF_URL/root.json"
$env:COSIGN_OIDC_CLIENT_ID = "trusted-artifact-signer"
$env:COSIGN_OIDC_ISSUER = $OIDC_ISSUER_URL
$env:COSIGN_CERTIFICATE_OIDC_ISSUER = $OIDC_ISSUER_URL
$env:COSIGN_YES = "true"
$env:SIGSTORE_FULCIO_URL = $COSIGN_FULCIO_URL
$env:SIGSTORE_OIDC_ISSUER = $OIDC_ISSUER_URL
$env:SIGSTORE_REKOR_URL = $COSIGN_REKOR_URL
$env:REKOR_REKOR_SERVER = $COSIGN_REKOR_URL
$env:SIGSTORE_OIDC_CLIENT_ID = "trusted-artifact-signer"

# Print the environment variables to verify they are set
Write-Output "TUF_URL: $env:TUF_URL"
Write-Output "OIDC_ISSUER_URL: $env:OIDC_ISSUER_URL"
Write-Output "COSIGN_FULCIO_URL: $env:COSIGN_FULCIO_URL"
Write-Output "COSIGN_REKOR_URL: $env:COSIGN_REKOR_URL"
Write-Output "COSIGN_MIRROR: $env:COSIGN_MIRROR"
Write-Output "COSIGN_ROOT: $env:COSIGN_ROOT"
Write-Output "COSIGN_OIDC_CLIENT_ID: $env:COSIGN_OIDC_CLIENT_ID"
Write-Output "COSIGN_OIDC_ISSUER: $env:COSIGN_OIDC_ISSUER"
Write-Output "COSIGN_CERTIFICATE_OIDC_ISSUER: $env:COSIGN_CERTIFICATE_OIDC_ISSUER"
Write-Output "COSIGN_YES: $env:COSIGN_YES"
Write-Output "SIGSTORE_FULCIO_URL: $env:SIGSTORE_FULCIO_URL"
Write-Output "SIGSTORE_OIDC_ISSUER: $env:SIGSTORE_OIDC_ISSUER"
Write-Output "SIGSTORE_REKOR_URL: $env:SIGSTORE_REKOR_URL"
Write-Output "REKOR_REKOR_SERVER: $env:REKOR_REKOR_SERVER"
Write-Output "SIGSTORE_OIDC_CLIENT_ID: $env:SIGSTORE_OIDC_CLIENT_ID"
