#!/bin/bash
# Export the environment variables for the current session
export TUF_URL=$(oc get tuf -o jsonpath='{.items[0].status.url}' -n trusted-artifact-signer)
export OIDC_ISSUER_URL=https://$(oc get route keycloak -n keycloak-system | tail -n 1 | awk '{print $2}')/auth/realms/trusted-artifact-signer
export COSIGN_FULCIO_URL=$(oc get fulcio -o jsonpath='{.items[0].status.url}' -n trusted-artifact-signer)
export COSIGN_REKOR_URL=$(oc get rekor -o jsonpath='{.items[0].status.url}' -n trusted-artifact-signer)
export REKOR_UI_URL=$(oc get rekor -o jsonpath='{.items[0].status.rekorSearchUIUrl}' -n trusted-artifact-signer)
export COSIGN_MIRROR=$TUF_URL
export COSIGN_ROOT=$TUF_URL/root.json
export COSIGN_OIDC_CLIENT_ID="trusted-artifact-signer"
export COSIGN_OIDC_ISSUER=$OIDC_ISSUER_URL
export COSIGN_CERTIFICATE_OIDC_ISSUER=$OIDC_ISSUER_URL
export COSIGN_YES="true"
export SIGSTORE_FULCIO_URL=$COSIGN_FULCIO_URL
export SIGSTORE_OIDC_ISSUER=$COSIGN_OIDC_ISSUER
export SIGSTORE_REKOR_URL=$COSIGN_REKOR_URL
export SIGSTORE_REKOR_UI_URL=$REKOR_UI_URL
export REKOR_REKOR_SERVER=$COSIGN_REKOR_URL
export SIGSTORE_OIDC_CLIENT_ID=trusted-artifact-signer

# Print the environment variables to verify they are set
echo "export TUF_URL=$TUF_URL"
echo "export OIDC_ISSUER_URL=$OIDC_ISSUER_URL"
echo "export COSIGN_FULCIO_URL=$COSIGN_FULCIO_URL"
echo "export COSIGN_REKOR_URL=$COSIGN_REKOR_URL"
echo "export COSIGN_MIRROR=$COSIGN_MIRROR"
echo "export COSIGN_ROOT=$COSIGN_ROOT"
echo "export COSIGN_OIDC_CLIENT_ID=$COSIGN_OIDC_CLIENT_ID"
echo "export COSIGN_OIDC_ISSUER=$COSIGN_OIDC_ISSUER"
echo "export COSIGN_CERTIFICATE_OIDC_ISSUER=$COSIGN_CERTIFICATE_OIDC_ISSUER"
echo "export COSIGN_YES=$COSIGN_YES"
echo "export SIGSTORE_FULCIO_URL=$SIGSTORE_FULCIO_URL"
echo "export SIGSTORE_OIDC_ISSUER=$SIGSTORE_OIDC_ISSUER"
echo "export SIGSTORE_REKOR_URL=$SIGSTORE_REKOR_URL"
echo "export REKOR_REKOR_SERVER=$REKOR_REKOR_SERVER"
echo "export SIGSTORE_OIDC_CLIENT_ID=$SIGSTORE_OIDC_CLIENT_ID"
echo "export SIGSTORE_REKOR_UI_URL=$SIGSTORE_REKOR_UI_URL"
echo "export REKOR_UI_URL=$REKOR_UI_URL"

