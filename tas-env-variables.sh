#!/bin/bash

if [ -z "$OIDC_ISSUER_URL" ]; then
  export OIDC_ISSUER_URL=https://$(oc get route -n keycloak-system -l app=keycloak -o jsonpath='{.items[0].status.ingress[0].host}')/realms/trusted-artifact-signer
fi

if [ -z "$OIDC_CLIENT_ID" ]; then
  OIDC_CLIENT_ID="trusted-artifact-signer"
fi

if [ -z "$TUF_URL" ]; then
  export TUF_URL=$(oc get tuf -o jsonpath='{.items[0].status.url}')
fi

if [ -z "$FULCIO_URL" ]; then
  export FULCIO_URL=$(oc get fulcio -o jsonpath='{.items[0].status.url}')
fi

if [ -z "$REKOR_URL" ]; then
  export REKOR_URL=$(oc get rekor -o jsonpath='{.items[0].status.url}')
fi

if [ -z "$REKOR_UI_URL" ]; then
  export REKOR_UI_URL=$(oc get rekor -o jsonpath='{.items[0].status.rekorSearchUIUrl}')
fi

# Export the environment variables for the current session
export REKOR_REKOR_SERVER=$REKOR_URL
export SIGSTORE_REKOR_UI_URL=$REKOR_UI_URL
export COSIGN_MIRROR=$TUF_URL
export COSIGN_ROOT=$TUF_URL/root.json
export COSIGN_OIDC_CLIENT_ID=$OIDC_CLIENT_ID
export SIGSTORE_OIDC_ISSUER=$OIDC_ISSUER_URL
export SIGSTORE_OIDC_CLIENT_ID=$OIDC_CLIENT_ID
export COSIGN_YES="true"

# Print the environment variables to verify they are set
echo "TUF_URL=$TUF_URL"
echo "SIGSTORE_REKOR_UI_URL=$SIGSTORE_REKOR_UI_URL"
echo "REKOR_UI_URL=$REKOR_UI_URL"
echo "OIDC_ISSUER_URL=$OIDC_ISSUER_URL"
echo "COSIGN_MIRROR=$COSIGN_MIRROR"
echo "COSIGN_ROOT=$COSIGN_ROOT"
echo "COSIGN_OIDC_CLIENT_ID=$COSIGN_OIDC_CLIENT_ID"
echo "COSIGN_YES=$COSIGN_YES"
echo "SIGSTORE_OIDC_ISSUER=$SIGSTORE_OIDC_ISSUER"
echo "SIGSTORE_OIDC_CLIENT_ID=$SIGSTORE_OIDC_CLIENT_ID"
echo "FULCIO_URL=$FULCIO_URL"
echo "REKOR_URL=$REKOR_URL"
echo "REKOR_REKOR_SERVER=$REKOR_REKOR_SERVER"
