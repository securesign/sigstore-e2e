@echo off

REM Get the URLs and export them as environment variables
for /f "tokens=*" %%i in ('oc get tuf -o jsonpath^="{.items[0].status.url}"') do set TUF_URL=%%i
for /f "tokens=*" %%i in ('oc get route -n keycloak-system -l app^=keycloak -o jsonpath^="{.items[0].spec.host}"') do set OIDC_ROUTE=%%i
set OIDC_ISSUER_URL=https://%OIDC_ROUTE%/realms/trusted-artifact-signer
for /f "tokens=*" %%i in ('oc get fulcio -o jsonpath^="{.items[0].status.url}"') do set FULCIO_URL=%%i
for /f "tokens=*" %%i in ('oc get rekor -o jsonpath^="{.items[0].status.url}"') do set REKOR_URL=%%i
for /f "tokens=*" %%i in ('oc get rekor -o jsonpath^="{.items[0].status.rekorSearchUIUrl}"') do set REKOR_UI_URL=%%i

if "%OIDC_CLIENT_ID%"=="" set OIDC_CLIENT_ID=trusted-artifact-signer

REM Export the environment variables for the current session
set REKOR_REKOR_SERVER=%REKOR_URL%
set SIGSTORE_REKOR_UI_URL=%REKOR_UI_URL%
set COSIGN_MIRROR=%TUF_URL%
set COSIGN_ROOT=%TUF_URL%/root.json
set COSIGN_OIDC_CLIENT_ID=%OIDC_CLIENT_ID%
set SIGSTORE_OIDC_CLIENT_ID=%OIDC_CLIENT_ID%
set COSIGN_YES=true

REM Print the environment variables to verify they are set
echo TUF_URL=%TUF_URL%
echo SIGSTORE_REKOR_UI_URL=%SIGSTORE_REKOR_UI_URL%
echo REKOR_UI_URL=%REKOR_UI_URL%
echo OIDC_ISSUER_URL=%OIDC_ISSUER_URL%
echo COSIGN_MIRROR=%COSIGN_MIRROR%
echo COSIGN_ROOT=%COSIGN_ROOT%
echo COSIGN_OIDC_CLIENT_ID=%COSIGN_OIDC_CLIENT_ID%
echo COSIGN_YES=%COSIGN_YES%
echo SIGSTORE_OIDC_CLIENT_ID=%SIGSTORE_OIDC_CLIENT_ID%
echo FULCIO_URL=%FULCIO_URL%
echo REKOR_URL=%REKOR_URL%
echo REKOR_REKOR_SERVER=%REKOR_REKOR_SERVER%
