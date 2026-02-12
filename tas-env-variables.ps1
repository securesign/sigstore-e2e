# Get the URLs and export them as environment variables
$TUF_URL = $(oc get tuf -o jsonpath='{.items[0].status.url}')
$OIDC_ROUTE = $(oc get route keycloak -n keycloak-system --template='{{.spec.host}}')
$OIDC_ISSUER_URL = "https://$OIDC_ROUTE/auth/realms/trusted-artifact-signer"


if (-not $env:OIDC_CLIENT_ID) {
    $OIDC_CLIENT_ID = "trusted-artifact-signer"
} else {
    $OIDC_CLIENT_ID = $env:OIDC_CLIENT_ID
}

# Export the environment variables for the current session
$env:TUF_URL = $TUF_URL
$env:COSIGN_MIRROR = $TUF_URL
$env:COSIGN_ROOT = "$TUF_URL/root.json"
$env:COSIGN_OIDC_CLIENT_ID = $OIDC_CLIENT_ID


# Print the environment variables to verify they are set
Write-Output "TUF_URL=$env:TUF_URL"
Write-Output "COSIGN_MIRROR=$env:COSIGN_MIRROR"
Write-Output "COSIGN_ROOT=$env:COSIGN_ROOT"
Write-Output "COSIGN_OIDC_CLIENT_ID=$env:COSIGN_OIDC_CLIENT_ID"

