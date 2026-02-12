@echo off

REM Get the URLs and export them as environment variables
for /f "tokens=*" %%i in ('oc get tuf -o jsonpath^="{.items[0].status.url}"') do set TUF_URL=%%i
for /f "tokens=*" %%i in ('oc get route keycloak -n keycloak-system --template^="{{.spec.host}}"') do set OIDC_ROUTE=%%i
set OIDC_ISSUER_URL=https://%OIDC_ROUTE%/auth/realms/trusted-artifact-signer
for /f "tokens=*" %%i in ('oc get fulcio -o jsonpath^="{.items[0].status.url}"') do set FULCIO_URL=%%i
for /f "tokens=*" %%i in ('oc get rekor -o jsonpath^="{.items[0].status.url}"') do set REKOR_URL=%%i
for /f "tokens=*" %%i in ('oc get rekor -o jsonpath^="{.items[0].status.rekorSearchUIUrl}"') do set REKOR_UI_URL=%%i

if "%OIDC_CLIENT_ID%"=="" set OIDC_CLIENT_ID=trusted-artifact-signer

REM Export the environment variables for the current session
set COSIGN_MIRROR=%TUF_URL%
set COSIGN_ROOT=%TUF_URL%/root.json
set COSIGN_OIDC_CLIENT_ID=%OIDC_CLIENT_ID%

REM Print the environment variables to verify they are set
echo TUF_URL=%TUF_URL%
echo COSIGN_MIRROR=%COSIGN_MIRROR%
echo COSIGN_ROOT=%COSIGN_ROOT%
echo COSIGN_OIDC_CLIENT_ID=%COSIGN_OIDC_CLIENT_ID%


