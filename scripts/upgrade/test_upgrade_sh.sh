#!/usr/bin/env bash

# Sandbox dry-run for upgrade.sh. Builds a fake 1Panel environment under a temp
# dir (fake BASE_DIR, fake /usr/local/bin, fake systemctl shim that records the
# call sequence, and a correctly-layered tar package) and exercises the normal
# upgrade, --rollback and --no-db-fix paths. Repeatable; CI need not run it.
#
#   bash scripts/upgrade/test_upgrade_sh.sh

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPGRADE_SH="${HERE}/upgrade.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

PASS=0
FAIL=0

assert_eq() {
    local desc="$1" want="$2" got="$3"
    if [[ "${want}" == "${got}" ]]; then
        printf 'PASS: %s\n' "${desc}"
        PASS=$((PASS + 1))
    else
        printf 'FAIL: %s\n      want=[%s]\n      got =[%s]\n' "${desc}" "${want}" "${got}"
        FAIL=$((FAIL + 1))
    fi
}

assert_true() {
    local desc="$1" cond="$2"
    if [[ "${cond}" == "1" ]]; then
        printf 'PASS: %s\n' "${desc}"
        PASS=$((PASS + 1))
    else
        printf 'FAIL: %s\n' "${desc}"
        FAIL=$((FAIL + 1))
    fi
}

# Globals set by setup_env.
BASE=""; BIN=""; PKG=""; CALL_LOG=""; FAKE_CTL=""

setup_env() {
    local root="${WORK}/$1"
    rm -rf "${root}"
    BASE="${root}/base"
    BIN="${root}/bin"
    PKG="${root}/pkg"
    mkdir -p "${BASE}/1panel/conf" "${BASE}/1panel/db" "${BIN}" "${PKG}" "${root}/stage"

    printf 'BASE_DIR=%s\n' "${BASE}" > "${BASE}/1panel/conf/1pctl.env"

    sqlite3 "${BASE}/1panel/db/agent.db" \
        "CREATE TABLE windows_services(id INTEGER PRIMARY KEY, name TEXT, service_type TEXT);
         INSERT INTO windows_services(name, service_type) VALUES('svc1','jerinte');"

    printf 'OLD-CORE'  > "${BIN}/1panel-core";  chmod +x "${BIN}/1panel-core"
    printf 'OLD-AGENT' > "${BIN}/1panel-agent"; chmod +x "${BIN}/1panel-agent"

    local stage="${root}/stage/1panel-vTEST-linux-amd64"
    mkdir -p "${stage}"
    printf 'NEW-CORE-vTEST'  > "${stage}/1panel-core"
    printf 'NEW-AGENT-vTEST' > "${stage}/1panel-agent"
    printf 'vTEST\n' > "${stage}/VERSION"
    cp "${UPGRADE_SH}" "${stage}/upgrade.sh"
    tar -czf "${PKG}/1panel-vTEST-linux-amd64.tar.gz" -C "${root}/stage" "1panel-vTEST-linux-amd64"

    # Copy the script into the package dir so default package resolution finds
    # the tarball as "newest 1panel-*-linux-*.tar.gz next to the script".
    cp "${UPGRADE_SH}" "${PKG}/upgrade.sh"

    CALL_LOG="${root}/systemctl.log"
    : > "${CALL_LOG}"
    FAKE_CTL="${root}/fake-systemctl"
    cat > "${FAKE_CTL}" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "${CALL_LOG}"
if [[ "\$1" == "is-active" ]]; then echo active; fi
exit 0
EOF
    chmod +x "${FAKE_CTL}"
}

run_upgrade() {
    PANEL_SKIP_ROOT_CHECK=1 PANEL_BIN_DIR="${BIN}" PANEL_SERVICE_CTL="${FAKE_CTL}" \
        bash "${PKG}/upgrade.sh" --install-dir "${BASE}" "$@"
}

svc_sequence() {
    grep -E '^(stop|start) ' "${CALL_LOG}" | tr '\n' '|'
}

echo "================ Scenario 1: normal upgrade ================"
setup_env s1
OUT="$(run_upgrade 2>&1)"
echo "${OUT}"
echo "---- assertions ----"
assert_eq "core binary replaced" "NEW-CORE-vTEST"  "$(cat "${BIN}/1panel-core")"
assert_eq "agent binary replaced" "NEW-AGENT-vTEST" "$(cat "${BIN}/1panel-agent")"
assert_eq "service call sequence" "stop 1panel-agent|stop 1panel-core|start 1panel-core|start 1panel-agent|" "$(svc_sequence)"
assert_eq "jerinte migrated to package" "package" "$(sqlite3 "${BASE}/1panel/db/agent.db" "SELECT service_type FROM windows_services WHERE name='svc1';")"
assert_true "upgrade logged 1 row updated" "$(echo "${OUT}" | grep -q 'rows updated: 1' && echo 1 || echo 0)"
BK="$(ls -1d "${BASE}/1panel-upgrade-backup"/*/ 2>/dev/null | head -n1 || true)"
assert_true "backup dir created" "$([[ -n "${BK}" ]] && echo 1 || echo 0)"
assert_eq "backup holds old core" "OLD-CORE" "$(cat "${BK}bin/1panel-core" 2>/dev/null || echo MISSING)"
assert_true "backup holds conf" "$([[ -f "${BK}1panel/conf/1pctl.env" ]] && echo 1 || echo 0)"
assert_true "backup holds db" "$([[ -f "${BK}1panel/db/agent.db" ]] && echo 1 || echo 0)"

echo
echo "================ Scenario 2: rollback (--restore-data) ================"
setup_env s2
run_upgrade >/dev/null 2>&1
# Reset call log so we only capture the rollback sequence.
: > "${CALL_LOG}"
OUT="$(run_upgrade --rollback --restore-data --yes 2>&1)"
echo "${OUT}"
echo "---- assertions ----"
assert_eq "core binary rolled back" "OLD-CORE"  "$(cat "${BIN}/1panel-core")"
assert_eq "agent binary rolled back" "OLD-AGENT" "$(cat "${BIN}/1panel-agent")"
assert_eq "rollback call sequence" "stop 1panel-agent|stop 1panel-core|start 1panel-core|start 1panel-agent|" "$(svc_sequence)"
# Backup db predates the jerinte fix, so restore brings 'jerinte' back.
assert_eq "restore-data reverts db to jerinte" "jerinte" "$(sqlite3 "${BASE}/1panel/db/agent.db" "SELECT service_type FROM windows_services WHERE name='svc1';")"

echo
echo "================ Scenario 3: --no-db-fix ================"
setup_env s3
OUT="$(run_upgrade --no-db-fix 2>&1)"
echo "${OUT}"
echo "---- assertions ----"
assert_eq "binary still replaced" "NEW-CORE-vTEST" "$(cat "${BIN}/1panel-core")"
assert_eq "db untouched (still jerinte)" "jerinte" "$(sqlite3 "${BASE}/1panel/db/agent.db" "SELECT service_type FROM windows_services WHERE name='svc1';")"
assert_true "logged skip db fix" "$(echo "${OUT}" | grep -q 'skip db fix' && echo 1 || echo 0)"

echo
echo "================ Scenario 4: ambiguous package rejected ================"
setup_env s4
# Second copy of the core binary at another depth makes the package ambiguous.
mkdir -p "${WORK}/s4/stage/1panel-vTEST-linux-amd64/extra"
printf 'ROGUE' > "${WORK}/s4/stage/1panel-vTEST-linux-amd64/extra/1panel-core"
tar -czf "${PKG}/1panel-vTEST-linux-amd64.tar.gz" -C "${WORK}/s4/stage" "1panel-vTEST-linux-amd64"
OUT="$(run_upgrade 2>&1 || true)"
echo "${OUT}"
echo "---- assertions ----"
assert_true "ambiguous package is rejected" "$(echo "${OUT}" | grep -q 'ambiguous package' && echo 1 || echo 0)"
assert_eq "binaries untouched on rejection" "OLD-CORE" "$(cat "${BIN}/1panel-core")"

echo
echo "================ Scenario 5: BASE_DIR quoting variants ================"
setup_env s5
# detect_install_dir reads /usr/local/bin/1pctl -> point PANEL_BIN_DIR at a fake
# bin dir carrying a quoted export line, and drop --install-dir to force detection.
printf "export BASE_DIR='%s'\n" "${BASE}" > "${BIN}/1pctl"
OUT="$(PANEL_SKIP_ROOT_CHECK=1 PANEL_BIN_DIR="${BIN}" PANEL_SERVICE_CTL="${FAKE_CTL}" bash "${PKG}/upgrade.sh" 2>&1)"
echo "${OUT}"
echo "---- assertions ----"
assert_true "detected quoted export BASE_DIR" "$(echo "${OUT}" | grep -q "install dir: ${BASE}" && echo 1 || echo 0)"
assert_eq "upgrade completed via detection" "NEW-CORE-vTEST" "$(cat "${BIN}/1panel-core")"

echo
echo "================ Scenario 6: schema surprise degrades to warning ================"
setup_env s6
sqlite3 "${BASE}/1panel/db/agent.db" "DROP TABLE windows_services;"
OUT="$(run_upgrade 2>&1)"
echo "${OUT}"
echo "---- assertions ----"
assert_true "missing table logged as skip" "$(echo "${OUT}" | grep -q 'not present, skip jerinte fix' && echo 1 || echo 0)"
assert_eq "upgrade still completed" "NEW-CORE-vTEST" "$(cat "${BIN}/1panel-core")"
assert_true "services restarted" "$(svc_sequence | grep -q 'start 1panel-agent' && echo 1 || echo 0)"

echo
echo "================ Result ================"
printf 'PASS=%d FAIL=%d\n' "${PASS}" "${FAIL}"
[[ "${FAIL}" -eq 0 ]]
