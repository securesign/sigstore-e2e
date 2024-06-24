REM Get the URLs and export them as environment variables
for /f "tokens=*" %i in ('oc get tuf -o jsonpath^="{.items[0].status.url}" -n trusted-artifact-signer') do set TUF_URL=%i
for /f "tokens=*" %i in ('oc get route keycloak -n keycloak-system --template^="{{.spec.host}}"') do set OIDC_ROUTE=%i
set OIDC_ISSUER_URL=https://%OIDC_ROUTE%/auth/realms/trusted-artifact-signer
for /f "tokens=*" %i in ('oc get fulcio -o jsonpath^="{.items[0].status.url}" -n trusted-artifact-signer') do set COSIGN_FULCIO_URL=%i
for /f "tokens=*" %i in ('oc get rekor -o jsonpath^="{.items[0].status.url}" -n trusted-artifact-signer') do set COSIGN_REKOR_URL=%i

REM Print the URLs
echo TUF_URL: %TUF_URL%
echo OIDC_ISSUER_URL: %OIDC_ISSUER_URL%
echo COSIGN_FULCIO_URL: %COSIGN_FULCIO_URL%
echo COSIGN_REKOR_URL: %COSIGN_REKOR_URL%

REM Export the environment variables for the current session
set COSIGN_MIRROR=%TUF_URL%
set COSIGN_ROOT=%TUF_URL%/root.json
set COSIGN_OIDC_CLIENT_ID=trusted-artifact-signer
set COSIGN_OIDC_ISSUER=%OIDC_ISSUER_URL%
set COSIGN_CERTIFICATE_OIDC_ISSUER=%OIDC_ISSUER_URL%
set COSIGN_YES=true
set SIGSTORE_FULCIO_URL=%COSIGN_FULCIO_URL%
set SIGSTORE_OIDC_ISSUER=%OIDC_ISSUER_URL%
set SIGSTORE_REKOR_URL=%COSIGN_REKOR_URL%
set REKOR_REKOR_SERVER=%COSIGN_REKOR_URL%
set SIGSTORE_OIDC_CLIENT_ID=trusted-artifact-signer

REM Print the environment variables to verify they are set
echo TUF_URL: %TUF_URL%
echo OIDC_ISSUER_URL: %OIDC_ISSUER_URL%
echo COSIGN_FULCIO_URL: %COSIGN_FULCIO_URL%
echo COSIGN_REKOR_URL: %COSIGN_REKOR_URL%
echo COSIGN_MIRROR: %COSIGN_MIRROR%
echo COSIGN_ROOT: %COSIGN_ROOT%
echo COSIGN_OIDC_CLIENT_ID: %COSIGN_OIDC_CLIENT_ID%
echo COSIGN_OIDC_ISSUER: %COSIGN_OIDC_ISSUER%
echo COSIGN_CERTIFICATE_OIDC_ISSUER: %COSIGN_CERTIFICATE_OIDC_ISSUER%
echo COSIGN_YES: %COSIGN_YES%
echo SIGSTORE_FULCIO_URL: %SIGSTORE_FULCIO_URL%
echo SIGSTORE_OIDC_ISSUER: %SIGSTORE_OIDC_ISSUER%
echo SIGSTORE_REKOR_URL: %SIGSTORE_REKOR_URL%
echo REKOR_REKOR_SERVER: %REKOR_REKOR_SERVER%
echo SIGSTORE_OIDC_CLIENT_ID: %SIGSTORE_OIDC_CLIENT_ID%
