#!/usr/bin/env bash

set -euo pipefail

# ----------------------------------------------------------------------------
# 1Panel Linux upgrade script.
#
# Stops the running services, backs up the current binaries and data, swaps in
# the binaries from a release package (bare binaries next to this script, or the
# newest 1panel-*-linux-*.tar.gz), fixes the legacy 'jerinte' service_type rows,
# then starts the services again. Supports --rollback to a previous backup.
#
# Test/override hooks (default to production paths, override only for sandbox):
#   PANEL_BIN_DIR         binary install dir            (default /usr/local/bin)
#   PANEL_SERVICE_CTL     service controller executable (default systemctl)
#   PANEL_SKIP_ROOT_CHECK set to 1 to bypass the root check
# ----------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PANEL_BIN_DIR="${PANEL_BIN_DIR:-/usr/local/bin}"
PANEL_SERVICE_CTL="${PANEL_SERVICE_CTL:-systemctl}"

CORE_SERVICE="1panel-core"
AGENT_SERVICE="1panel-agent"
CORE_BIN="1panel-core"
AGENT_BIN="1panel-agent"

INSTALL_DIR=""
PACKAGE=""
SKIP_BACKUP="false"
FULL_BACKUP="false"
ROLLBACK="false"
RESTORE_DATA="false"
NO_DB_FIX="false"
ASSUME_YES="false"

LAST_BACKUP_DIR=""
NEW_VERSION="unknown"
TMP_DIR=""

usage() {
    cat <<'EOF'
Usage:
  sudo ./upgrade.sh [options]

Options:
  --install-dir DIR   1Panel BASE_DIR. Default: auto-detected
  --package PATH      Release package (tar.gz) or dir with bare binaries.
                      Default: bare binaries next to this script, else newest
                      1panel-*-linux-*.tar.gz in the script directory
  --skip-backup       Do not back up before replacing binaries
  --full-backup       Back up the whole 1panel data dir (not just conf + db)
  --rollback          Restore binaries from the newest backup and restart
  --restore-data      With --rollback, also restore conf + db (asks to confirm)
  --no-db-fix         Skip the legacy 'jerinte' service_type migration
  --yes               Assume yes; do not prompt for confirmation
  -h, --help          Show help

Examples:
  sudo ./upgrade.sh
  sudo ./upgrade.sh --package /root/1panel-v2.1.11-linux-amd64.tar.gz
  sudo ./upgrade.sh --rollback --restore-data
EOF
}

die() {
    printf '%s\n' "$*" >&2
    exit 1
}

log() {
    printf '[1Panel Upgrade] %s\n' "$*"
}

run_cmd() {
    log "$*"
    "$@"
}

require_root() {
    if [[ "${PANEL_SKIP_ROOT_CHECK:-}" == "1" ]]; then
        return 0
    fi
    if [[ "${EUID}" -ne 0 ]]; then
        die "please run as root"
    fi
}

cleanup() {
    if [[ -n "${TMP_DIR}" && -d "${TMP_DIR}" ]]; then
        rm -rf "${TMP_DIR}"
    fi
}
trap cleanup EXIT

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --install-dir) INSTALL_DIR="$2"; shift 2 ;;
            --package) PACKAGE="$2"; shift 2 ;;
            --skip-backup) SKIP_BACKUP="true"; shift ;;
            --full-backup) FULL_BACKUP="true"; shift ;;
            --rollback) ROLLBACK="true"; shift ;;
            --restore-data) RESTORE_DATA="true"; shift ;;
            --no-db-fix) NO_DB_FIX="true"; shift ;;
            --yes) ASSUME_YES="true"; shift ;;
            -h|--help) usage; exit 0 ;;
            *) die "unknown argument: $1" ;;
        esac
    done
}

detect_install_dir() {
    local pctl="${PANEL_BIN_DIR}/1pctl"
    if [[ -f "${pctl}" ]]; then
        local base
        base="$(grep -m1 -E '^(export[[:space:]]+)?BASE_DIR=' "${pctl}" 2>/dev/null | sed -E 's/^(export[[:space:]]+)?BASE_DIR=//' | tr -d '"'"'"'' || true)"
        if [[ -n "${base}" && -f "${base}/1panel/conf/1pctl.env" ]]; then
            printf '%s\n' "${base}"
            return 0
        fi
    fi
    local cand
    for cand in /opt /usr/local /root /home/1panel; do
        if [[ -f "${cand}/1panel/conf/1pctl.env" ]]; then
            printf '%s\n' "${cand}"
            return 0
        fi
    done
    printf '%s\n' "/opt"
}

verify_install_dir() {
    [[ -n "${INSTALL_DIR}" ]] || die "install dir could not be determined"
    [[ -f "${INSTALL_DIR}/1panel/conf/1pctl.env" ]] \
        || die "not a 1Panel install dir (missing 1panel/conf/1pctl.env): ${INSTALL_DIR}"
}

# Sets NEW_CORE and NEW_AGENT to the resolved binary paths and NEW_VERSION.
resolve_package() {
    local src="${PACKAGE}"
    if [[ -z "${src}" ]]; then
        if [[ -f "${SCRIPT_DIR}/${CORE_BIN}" && -f "${SCRIPT_DIR}/${AGENT_BIN}" ]]; then
            src="${SCRIPT_DIR}"
        else
            src="$(ls -1t "${SCRIPT_DIR}"/1panel-*-linux-*.tar.gz 2>/dev/null | head -n1 || true)"
            [[ -n "${src}" ]] || die "no package found: pass --package or place binaries/tarball next to this script"
        fi
    fi

    if [[ -d "${src}" ]]; then
        NEW_CORE="${src}/${CORE_BIN}"
        NEW_AGENT="${src}/${AGENT_BIN}"
        [[ -f "${NEW_CORE}" && -f "${NEW_AGENT}" ]] || die "package dir missing binaries: ${src}"
        if [[ -f "${src}/VERSION" ]]; then
            NEW_VERSION="$(tr -d '[:space:]' < "${src}/VERSION")"
        fi
        log "using bare binaries from: ${src}"
        return 0
    fi

    [[ -f "${src}" ]] || die "package not found: ${src}"
    TMP_DIR="$(mktemp -d)"
    run_cmd tar -xzf "${src}" -C "${TMP_DIR}"
    # Strip the wrapping directory layer: locate binaries wherever they landed.
    # A well-formed package has exactly one of each; anything else is a bad package.
    local core_matches agent_matches
    core_matches="$(find "${TMP_DIR}" -type f -name "${CORE_BIN}" || true)"
    agent_matches="$(find "${TMP_DIR}" -type f -name "${AGENT_BIN}" || true)"
    [[ -n "${core_matches}" && -n "${agent_matches}" ]] || die "extracted package missing binaries: ${src}"
    [[ "$(printf '%s\n' "${core_matches}" | wc -l)" -eq 1 ]] || die "ambiguous package: multiple ${CORE_BIN} found in ${src}"
    [[ "$(printf '%s\n' "${agent_matches}" | wc -l)" -eq 1 ]] || die "ambiguous package: multiple ${AGENT_BIN} found in ${src}"
    NEW_CORE="${core_matches}"
    NEW_AGENT="${agent_matches}"

    local vfile
    vfile="$(find "${TMP_DIR}" -type f -name VERSION | head -n1 || true)"
    if [[ -n "${vfile}" ]]; then
        NEW_VERSION="$(tr -d '[:space:]' < "${vfile}")"
    else
        # Fallback: parse 1panel-<tag>-linux-<arch>.tar.gz
        local base
        base="$(basename "${src}")"
        if [[ "${base}" =~ ^1panel-(.+)-linux-[^-]+\.tar\.gz$ ]]; then
            NEW_VERSION="${BASH_REMATCH[1]}"
        fi
    fi
    log "extracted package: ${src} (version: ${NEW_VERSION})"
}

svc_stop() {
    local svc="$1"
    if ! run_cmd "${PANEL_SERVICE_CTL}" stop "${svc}"; then
        log "WARNING: failed to stop ${svc} (continuing)"
    fi
}

svc_start() {
    local svc="$1"
    run_cmd "${PANEL_SERVICE_CTL}" start "${svc}"
}

wait_active() {
    local svc="$1" i state
    for ((i = 0; i < 60; i++)); do
        state="$("${PANEL_SERVICE_CTL}" is-active "${svc}" 2>/dev/null || true)"
        if [[ "${state}" == "active" ]]; then
            log "service active: ${svc}"
            return 0
        fi
        sleep 1
    done
    log "WARNING: service did not reach active state within 60s: ${svc}"
    return 1
}

do_backup() {
    if [[ "${SKIP_BACKUP}" == "true" ]]; then
        log "skip backup (--skip-backup)"
        return 0
    fi
    local ts dest
    ts="$(date +%Y%m%d-%H%M%S)"
    dest="${INSTALL_DIR}/1panel-upgrade-backup/${ts}"
    mkdir -p "${dest}/bin"
    if [[ -f "${PANEL_BIN_DIR}/${CORE_BIN}" ]]; then
        cp -a "${PANEL_BIN_DIR}/${CORE_BIN}" "${dest}/bin/"
    fi
    if [[ -f "${PANEL_BIN_DIR}/${AGENT_BIN}" ]]; then
        cp -a "${PANEL_BIN_DIR}/${AGENT_BIN}" "${dest}/bin/"
    fi
    if [[ "${FULL_BACKUP}" == "true" ]]; then
        mkdir -p "${dest}/1panel"
        cp -a "${INSTALL_DIR}/1panel/." "${dest}/1panel/"
    else
        mkdir -p "${dest}/1panel"
        [[ -d "${INSTALL_DIR}/1panel/conf" ]] && cp -a "${INSTALL_DIR}/1panel/conf" "${dest}/1panel/"
        [[ -d "${INSTALL_DIR}/1panel/db" ]] && cp -a "${INSTALL_DIR}/1panel/db" "${dest}/1panel/"
    fi
    LAST_BACKUP_DIR="${dest}"
    log "backup created: ${dest}"
}

replace_binaries() {
    run_cmd install -m 755 "${NEW_CORE}" "${PANEL_BIN_DIR}/${CORE_BIN}.new"
    run_cmd mv -f "${PANEL_BIN_DIR}/${CORE_BIN}.new" "${PANEL_BIN_DIR}/${CORE_BIN}"
    run_cmd install -m 755 "${NEW_AGENT}" "${PANEL_BIN_DIR}/${AGENT_BIN}.new"
    run_cmd mv -f "${PANEL_BIN_DIR}/${AGENT_BIN}.new" "${PANEL_BIN_DIR}/${AGENT_BIN}"
    log "binaries replaced"
}

db_fix() {
    if [[ "${NO_DB_FIX}" == "true" ]]; then
        log "skip db fix (--no-db-fix)"
        return 0
    fi
    local db="${INSTALL_DIR}/1panel/db/agent.db"
    if [[ ! -f "${db}" ]]; then
        log "db not found, skip jerinte fix: ${db}"
        return 0
    fi
    if command -v sqlite3 >/dev/null 2>&1; then
        # Never abort the upgrade here: services are stopped and binaries already
        # replaced, so a schema surprise must degrade to a warning, not a die.
        local has_table
        if ! has_table="$(sqlite3 "${db}" "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='windows_services';" 2>&1)"; then
            log "WARNING: sqlite3 could not read db (${has_table}); skip jerinte fix"
            return 0
        fi
        if [[ "${has_table}" != "1" ]]; then
            log "table windows_services not present, skip jerinte fix"
            return 0
        fi
        local changed
        if ! changed="$(sqlite3 "${db}" "UPDATE windows_services SET service_type='package' WHERE service_type='jerinte'; SELECT changes();" 2>&1)"; then
            log "WARNING: jerinte fix failed (${changed}); run manually: sqlite3 \"${db}\" \"UPDATE windows_services SET service_type='package' WHERE service_type='jerinte';\""
            return 0
        fi
        log "jerinte -> package rows updated: ${changed}"
    else
        if grep -qa jerinte "${db}"; then
            log "WARNING: sqlite3 not found but 'jerinte' bytes present in ${db}"
            log "Run manually: sqlite3 \"${db}\" \"UPDATE windows_services SET service_type='package' WHERE service_type='jerinte';\""
        else
            log "sqlite3 not found; no 'jerinte' bytes detected, skip"
        fi
    fi
}

# sync_version records the new version so the panel footer reflects it. The
# upgraded core binary self-heals SystemVersion in core.db on startup (compiled
# version injected via ldflags), so this is belt-and-suspenders: it updates
# ORIGINAL_VERSION in the param file (from which the agent re-derives its version
# each start) and, when sqlite3 is present, writes SystemVersion immediately.
sync_version() {
    local ver="${NEW_VERSION}"
    if [[ -z "${ver}" || "${ver}" == "unknown" ]]; then
        log "version unknown, skip version sync"
        return 0
    fi
    local pctl="${PANEL_BIN_DIR}/1pctl"
    if [[ -f "${pctl}" ]]; then
        if grep -qE '^ORIGINAL_VERSION=' "${pctl}"; then
            sed -i -E "s|^ORIGINAL_VERSION=.*|ORIGINAL_VERSION=${ver}|" "${pctl}" \
                && log "updated ORIGINAL_VERSION in ${pctl}: ${ver}" \
                || log "WARNING: failed to update ORIGINAL_VERSION in ${pctl}"
        fi
    fi
    if command -v sqlite3 >/dev/null 2>&1; then
        local p
        for p in "${INSTALL_DIR}/1panel/db/core.db" "${INSTALL_DIR}/1panel/db/agent.db"; do
            [[ -f "${p}" ]] || continue
            if ! sqlite3 "${p}" "UPDATE settings SET value='${ver}' WHERE key='SystemVersion';" 2>/dev/null; then
                log "WARNING: could not update SystemVersion in ${p}"
            fi
        done
        log "SystemVersion set to ${ver} in core.db/agent.db"
    else
        log "sqlite3 not found; core binary will self-heal SystemVersion on next start"
    fi
}

confirm_yes() {
    local prompt="$1"
    if [[ "${ASSUME_YES}" == "true" ]]; then
        return 0
    fi
    local answer=""
    printf '%s [type YES to confirm]: ' "${prompt}"
    read -r answer || true
    [[ "${answer}" == "YES" ]]
}

do_upgrade() {
    verify_install_dir
    resolve_package
    log "install dir: ${INSTALL_DIR}"
    log "new version: ${NEW_VERSION}"

    svc_stop "${AGENT_SERVICE}"
    svc_stop "${CORE_SERVICE}"

    do_backup
    replace_binaries
    db_fix
    sync_version

    svc_start "${CORE_SERVICE}"
    svc_start "${AGENT_SERVICE}"
    wait_active "${CORE_SERVICE}" || true
    wait_active "${AGENT_SERVICE}" || true

    log "===== upgrade summary ====="
    log "install dir : ${INSTALL_DIR}"
    log "new version : ${NEW_VERSION}"
    log "backup dir  : ${LAST_BACKUP_DIR:-<skipped>}"
    log "core binary : ${PANEL_BIN_DIR}/${CORE_BIN}"
    log "agent binary: ${PANEL_BIN_DIR}/${AGENT_BIN}"
    log "done"
}

do_rollback() {
    verify_install_dir
    local backup_root="${INSTALL_DIR}/1panel-upgrade-backup"
    [[ -d "${backup_root}" ]] || die "no backup dir: ${backup_root}"
    local latest
    latest="$(ls -1d "${backup_root}"/*/ 2>/dev/null | sort | tail -n1 || true)"
    latest="${latest%/}"
    [[ -n "${latest}" && -d "${latest}" ]] || die "no backup found under: ${backup_root}"
    log "rolling back from: ${latest}"

    svc_stop "${AGENT_SERVICE}"
    svc_stop "${CORE_SERVICE}"

    if [[ -f "${latest}/bin/${CORE_BIN}" ]]; then
        run_cmd install -m 755 "${latest}/bin/${CORE_BIN}" "${PANEL_BIN_DIR}/${CORE_BIN}"
    fi
    if [[ -f "${latest}/bin/${AGENT_BIN}" ]]; then
        run_cmd install -m 755 "${latest}/bin/${AGENT_BIN}" "${PANEL_BIN_DIR}/${AGENT_BIN}"
    fi

    if [[ "${RESTORE_DATA}" == "true" ]]; then
        if confirm_yes "Restore conf + db from backup? This overwrites current data"; then
            [[ -d "${latest}/1panel/conf" ]] && cp -a "${latest}/1panel/conf/." "${INSTALL_DIR}/1panel/conf/"
            [[ -d "${latest}/1panel/db" ]] && cp -a "${latest}/1panel/db/." "${INSTALL_DIR}/1panel/db/"
            log "data restored from: ${latest}"
        else
            log "data restore cancelled; binaries only"
        fi
    fi

    svc_start "${CORE_SERVICE}"
    svc_start "${AGENT_SERVICE}"
    wait_active "${CORE_SERVICE}" || true
    wait_active "${AGENT_SERVICE}" || true

    log "===== rollback summary ====="
    log "install dir : ${INSTALL_DIR}"
    log "restored from: ${latest}"
    log "data restored: ${RESTORE_DATA}"
    log "done"
}

main() {
    parse_args "$@"
    require_root
    if [[ -z "${INSTALL_DIR}" ]]; then
        INSTALL_DIR="$(detect_install_dir)"
    fi
    if [[ "${ROLLBACK}" == "true" ]]; then
        do_rollback
    else
        do_upgrade
    fi
}

main "$@"
