#!/bin/bash


if [ -z "$OIDC_CLIENT_ID" ]; then
  OIDC_CLIENT_ID="trusted-artifact-signer"
fi

if [ -z "$TUF_URL" ]; then
  export TUF_URL=$(oc get tuf -o jsonpath='{.items[0].status.url}')
fi


# Export the environment variables for the current session
export COSIGN_MIRROR=$TUF_URL
export COSIGN_ROOT=$TUF_URL/root.json
export COSIGN_OIDC_CLIENT_ID=$OIDC_CLIENT_ID

# Print the environment variables to verify they are set
echo "TUF_URL=$TUF_URL"
echo "COSIGN_MIRROR=$COSIGN_MIRROR"
echo "COSIGN_ROOT=$COSIGN_ROOT"
echo "COSIGN_OIDC_CLIENT_ID=$COSIGN_OIDC_CLIENT_ID"
