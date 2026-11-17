#!/usr/bin/env bash
#
# End-to-end test for terraform-provider-vault-secrets-rancher.
#
# Builds the sibling rancher plugin and this provider, boots Vault + a mock
# Rancher ext.cattle.io API in Docker, then runs a real `terraform apply` through
# the provider to mount the engine, seed a service account, and create a role.
# It asserts a token can be minted from the role, then `terraform destroy` and
# verifies the mount is gone.
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_REPO="${PLUGIN_REPO:-${ROOT_DIR}/../vault-plugin-secrets-rancher}"
COMPOSE_FILE="${ROOT_DIR}/test/docker-compose.yml"
COMPOSE="docker compose -f ${COMPOSE_FILE}"
TF="${TF:-terraform}"
E2E_DIR="${ROOT_DIR}/test/e2e"
PLUGIN_BIN="vault-plugin-secrets-rancher"
PROVIDER_BIN="terraform-provider-vault-secrets-rancher"

RANCHER_ADDR="http://127.0.0.1:8443"
export VAULT_ADDR="http://127.0.0.1:8200"
export VAULT_TOKEN="root"
export PLUGIN_SRC="${PLUGIN_REPO}"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
blue()  { printf '\033[34m==> %s\033[0m\n' "$*"; }
fail()  { red "FAIL: $*"; exit 1; }

cleanup() {
  blue "Cleaning up"
  if [ -d "${E2E_DIR}" ]; then
    (cd "${E2E_DIR}" && TF_CLI_CONFIG_FILE="${ROOT_DIR}/test/dev.tfrc" "${TF}" destroy -auto-approve >/dev/null 2>&1 || true)
    rm -f "${E2E_DIR}"/terraform.tfstate* "${E2E_DIR}"/.terraform.lock.hcl
    rm -rf "${E2E_DIR}/.terraform"
  fi
  ${COMPOSE} down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for() {
  local desc="$1"; shift
  local retries="${WAIT_RETRIES:-90}" i=0
  until "$@" >/dev/null 2>&1; do
    i=$((i + 1))
    [ "$i" -ge "$retries" ] && fail "timed out waiting for ${desc}"
    sleep 2
  done
  green "ready: ${desc}"
}

# ---------------------------------------------------------------------------
blue "Building rancher plugin from ${PLUGIN_REPO}"
[ -d "${PLUGIN_REPO}" ] || fail "plugin repo not found at ${PLUGIN_REPO} (set PLUGIN_REPO)"
mkdir -p "${ROOT_DIR}/test/plugins"
( cd "${PLUGIN_REPO}" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" \
    -o "${ROOT_DIR}/test/plugins/${PLUGIN_BIN}" ./cmd/vault-plugin-secrets-rancher )

blue "Building the provider"
( cd "${ROOT_DIR}" && go build -o "${PROVIDER_BIN}" . )

blue "Writing terraform dev_overrides config"
cat > "${ROOT_DIR}/test/dev.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/iuclabs/vault-secrets-rancher" = "${ROOT_DIR}"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="${ROOT_DIR}/test/dev.tfrc"

blue "Starting Docker stack"
${COMPOSE} up -d --build
wait_for "rancher" curl -fsS "${RANCHER_ADDR}/healthz"
wait_for "vault"   ${COMPOSE} exec -T vault vault status -address=http://127.0.0.1:8200

blue "Registering the rancher plugin in Vault"
SHA="$(sha256sum "${ROOT_DIR}/test/plugins/${PLUGIN_BIN}" | awk '{print $1}')"
${COMPOSE} exec -T vault vault plugin register -sha256="${SHA}" secret "${PLUGIN_BIN}" >/dev/null

# ---------------------------------------------------------------------------
blue "terraform apply (mount engine + service account + role via the provider)"
( cd "${E2E_DIR}" && "${TF}" apply -auto-approve )
green "apply succeeded"

blue "Verifying the mount, service account, and role exist"
${COMPOSE} exec -T vault vault secrets list 2>/dev/null | grep -q '^rancher/' \
  || fail "rancher mount not found after apply"
${COMPOSE} exec -T vault vault read rancher/service-accounts/admin >/dev/null \
  || fail "service account admin not found after apply"
${COMPOSE} exec -T vault vault read rancher/roles/ci >/dev/null \
  || fail "role ci not found after apply"
green "mount + service account + role present"

blue "Minting a token from the terraform-managed role"
TOKEN="$(${COMPOSE} exec -T vault vault read -field=token rancher/creds/ci)"
NAME="$(${COMPOSE} exec -T vault vault read -field=token_name rancher/creds/ci)"
[ -n "${TOKEN}" ] || fail "no token minted"
green "minted token ${TOKEN:0:14}... (${NAME})"

code="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${TOKEN}" \
  "${RANCHER_ADDR}/apis/ext.cattle.io/v1/tokens/${NAME}")"
[ "${code}" = "200" ] || fail "minted token not usable against rancher (HTTP ${code})"
green "minted token is live in rancher"

# ---------------------------------------------------------------------------
blue "terraform destroy (unmount engine)"
( cd "${E2E_DIR}" && "${TF}" destroy -auto-approve )
${COMPOSE} exec -T vault vault secrets list 2>/dev/null | grep -q '^rancher/' \
  && fail "rancher mount still present after destroy" || true
green "mount removed by destroy"

green "ALL PROVIDER END-TO-END CHECKS PASSED"
