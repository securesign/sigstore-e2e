# Get the URLs and export them as environment variables
$TUF_URL = $(oc get tuf -o jsonpath='{.items[0].status.url}')
$OIDC_ROUTE = $(oc get route -n keycloak-system -l app=keycloak -o jsonpath='{.items[0].spec.host}')
$OIDC_ISSUER_URL = "https://$OIDC_ROUTE/realms/trusted-artifact-signer"
$FULCIO_URL = $(oc get fulcio -o jsonpath='{.items[0].status.url}')
$REKOR_URL = $(oc get rekor -o jsonpath='{.items[0].status.url}')
$REKOR_UI_URL = $(oc get rekor -o jsonpath='{.items[0].status.rekorSearchUIUrl}')

if (-not $env:OIDC_CLIENT_ID) {
    $OIDC_CLIENT_ID = "trusted-artifact-signer"
} else {
    $OIDC_CLIENT_ID = $env:OIDC_CLIENT_ID
}

# Export the environment variables for the current session
$env:TUF_URL = $TUF_URL
$env:OIDC_ISSUER_URL = $OIDC_ISSUER_URL
$env:FULCIO_URL = $FULCIO_URL
$env:REKOR_URL = $REKOR_URL
$env:REKOR_UI_URL = $REKOR_UI_URL
$env:REKOR_REKOR_SERVER = $REKOR_URL
$env:SIGSTORE_REKOR_UI_URL = $REKOR_UI_URL
$env:COSIGN_MIRROR = $TUF_URL
$env:COSIGN_ROOT = "$TUF_URL/root.json"
$env:COSIGN_OIDC_CLIENT_ID = $OIDC_CLIENT_ID
$env:SIGSTORE_OIDC_ISSUER = $OIDC_ISSUER_URL
$env:SIGSTORE_OIDC_CLIENT_ID = $OIDC_CLIENT_ID
$env:COSIGN_YES = "true"

# Print the environment variables to verify they are set
Write-Output "TUF_URL=$env:TUF_URL"
Write-Output "SIGSTORE_REKOR_UI_URL=$env:SIGSTORE_REKOR_UI_URL"
Write-Output "REKOR_UI_URL=$env:REKOR_UI_URL"
Write-Output "OIDC_ISSUER_URL=$env:OIDC_ISSUER_URL"
Write-Output "COSIGN_MIRROR=$env:COSIGN_MIRROR"
Write-Output "COSIGN_ROOT=$env:COSIGN_ROOT"
Write-Output "COSIGN_OIDC_CLIENT_ID=$env:COSIGN_OIDC_CLIENT_ID"
Write-Output "COSIGN_YES=$env:COSIGN_YES"
Write-Output "SIGSTORE_OIDC_ISSUER=$env:SIGSTORE_OIDC_ISSUER"
Write-Output "SIGSTORE_OIDC_CLIENT_ID=$env:SIGSTORE_OIDC_CLIENT_ID"
Write-Output "FULCIO_URL=$env:FULCIO_URL"
Write-Output "REKOR_URL=$env:REKOR_URL"
Write-Output "REKOR_REKOR_SERVER=$env:REKOR_REKOR_SERVER"