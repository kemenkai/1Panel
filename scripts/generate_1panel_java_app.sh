#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PLACEHOLDER_LOGO="${REPO_DIR}/frontend/public/favicon.png"

APP_KEY=""
APP_NAME=""
APP_VERSION="1.0.0"
APP_TYPE="app"
APP_OUTPUT_ROOT="${REPO_DIR}/build/generated-local-apps"
SPEC_FILE=""
APP_JAR=""
APP_SERVER_NAME=""
APP_INTERNAL_PORT="8080"
APP_HOST_PORT="18080"
APP_JAVA_IMAGE="alibaba_dragonwell_jdk_anolis:21"
APP_TIMEZONE="Asia/Shanghai"
APP_JAVA_OPTS="-Xms512m -Xmx512m -XX:+HeapDumpOnOutOfMemoryError -Dfile.encoding=utf-8"
APP_APPLICATION_OPS=""
APP_CONFIG_FILE=""
APP_CONFIG_TARGET=""
APP_LOGO=""
APP_DESC_ZH=""
APP_DESC_EN=""
APP_MEMORY_REQUIRED="512"
ARCHIVE_OUTPUT="false"

WEB_DIR=""
WEB_ROUTE_PREFIX=""
WEB_API_PREFIX="/api/"
WEB_API_PROXY_PASS=""
WEB_ROOT_NAME=""

declare -a APP_TAGS=()
declare -a APP_ENVS=()
declare -a APP_FIXED_ENVS=()
declare -a FORM_OVERRIDES=()

usage() {
    cat <<'EOF'
Usage:
  generate_1panel_java_app.sh \
    --spec /path/to/service.spec.yaml

Or:
  generate_1panel_java_app.sh \
    --jar /path/to/app.jar \
    --app-key foo-service \
    --app-name "Foo Service" \
    [options]

Required:
  --spec PATH               YAML spec file for package generation
                           OR
  --jar PATH                 Path to the jar file
  --app-key KEY              1Panel app key
  --app-name NAME            1Panel app display name

Optional:
  --version VERSION          App version, default: 1.0.0
  --output-root DIR          Output root, default: build/generated-local-apps
  --server-name NAME         Jar file name inside container, default: app-key
  --internal-port PORT       Java service port inside container, default: 8080
  --host-port PORT           1Panel form default host port, default: 18080
  --java-image IMAGE         Default Java image
  --timezone TZ              Default timezone
  --java-opts OPTS           JAVA_OPS content
  --application-ops OPTS     APPLICATION_OPS content
  --config-file PATH         Extra config file mounted into container
  --config-target PATH       Target mount path for config file, default: /app/<basename>
  --logo PATH                App logo
  --desc-zh TEXT             Chinese description
  --desc-en TEXT             English description
  --tag TAG                  Repeatable app tag
  --env KEY=VALUE            Repeatable editable env var, auto-exposed in formFields
  --fixed-env KEY=VALUE      Repeatable hidden env var, written to .env only
  --memory-required MB       Memory required hint, default: 512
  --archive                  Also create <app-key>-<version>.tar.gz

Web options:
  --web-dir DIR              Optional static frontend directory
  --route-prefix PATH        Route prefix such as /drg, default: /<app-key>
  --api-prefix PATH          Nginx API prefix, default: /api/
  --api-proxy-pass URL       Nginx proxy_pass target, default: direct to api service
  --web-root-name NAME       Mounted web root name inside nginx

Examples:
  Spec-driven:
    generate_1panel_java_app.sh --spec ./service.spec.yaml

  Backend only:
    generate_1panel_java_app.sh \
      --jar ./foo.jar \
      --app-key foo-service \
      --app-name "Foo Service" \
      --internal-port 8088 \
      --host-port 18088 \
      --env REDIS_HOST=base-redis \
      --env REDIS_PORT=6379

  Backend + frontend:
    generate_1panel_java_app.sh \
      --jar ./foo.jar \
      --app-key foo-web \
      --app-name "Foo Web" \
      --internal-port 8080 \
      --host-port 18080 \
      --web-dir ./dist \
      --route-prefix /foo \
      --api-prefix /foo/api/ \
      --api-proxy-pass http://foo-web-api:8080/
EOF
}

die() {
    printf '%s\n' "$*" >&2
    exit 1
}

require_file() {
    local file="$1"
    [[ -f "${file}" ]] || die "missing file: ${file}"
}

escape_yaml() {
    printf '%s' "$1" | sed 's/"/\\"/g'
}

humanize_label() {
    local key="$1"
    key="${key//_/ }"
    key="$(printf '%s' "${key}" | tr '[:upper:]' '[:lower:]')"
    awk '{
        for (i=1; i<=NF; i++) {
            $i=toupper(substr($i,1,1)) substr($i,2)
        }
        print
    }' <<<"${key}"
}

quote_compose() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf '"%s"' "${value}"
}

find_form_override() {
    local target_key="$1"
    local item current_key
    CURRENT_FIELD_LABEL_ZH=""
    CURRENT_FIELD_LABEL_EN=""
    CURRENT_FIELD_TYPE=""
    CURRENT_FIELD_RULE=""
    CURRENT_FIELD_REQUIRED=""
    if ((${#FORM_OVERRIDES[@]})); then
        for item in "${FORM_OVERRIDES[@]}"; do
            IFS=$'\037' read -r current_key CURRENT_FIELD_LABEL_ZH CURRENT_FIELD_LABEL_EN CURRENT_FIELD_TYPE CURRENT_FIELD_RULE CURRENT_FIELD_REQUIRED <<<"${item}"
            if [[ "${current_key}" == "${target_key}" ]]; then
                return 0
            fi
        done
    fi
    return 1
}

resolve_required() {
    local value="$1"
    if [[ -z "${value}" ]]; then
        printf 'true\n'
        return 0
    fi
    printf '%s\n' "${value}"
}

load_spec_file() {
    local spec_path="$1"
    require_file "${spec_path}"
    local spec_dir
    spec_dir="$(cd "$(dirname "${spec_path}")" && pwd)"
    eval "$(
        SPEC_PATH="${spec_path}" SPEC_DIR="${spec_dir}" ruby <<'RUBY'
require 'yaml'
require 'pathname'
require 'shellwords'

def shq(value)
  Shellwords.escape(value.to_s)
end

def emit(key, value)
  puts "#{key}=#{shq(value)}"
end

spec_path = ENV['SPEC_PATH']
spec_dir = ENV['SPEC_DIR']
data = YAML.load_file(spec_path) || {}
app = data['app'] || {}
backend = data['backend'] || {}
web = data['web'] || {}
env = data['env'] || {}
form = data['form'] || {}

resolve = lambda do |path|
  next "" if path.nil? || path.to_s.strip.empty?
  Pathname.new(path.to_s).absolute? ? path.to_s : File.expand_path(path.to_s, spec_dir)
end

emit('APP_KEY', app['key']) if app.key?('key')
emit('APP_NAME', app['name']) if app.key?('name')
emit('APP_VERSION', app['version']) if app.key?('version')
emit('APP_TYPE', app['type']) if app.key?('type')
emit('APP_OUTPUT_ROOT', resolve.call(app['outputRoot'])) if app.key?('outputRoot')
emit('APP_LOGO', resolve.call(app['logo'])) if app.key?('logo')
emit('APP_DESC_ZH', app['descZh']) if app.key?('descZh')
emit('APP_DESC_EN', app['descEn']) if app.key?('descEn')
emit('APP_MEMORY_REQUIRED', app['memoryRequired']) if app.key?('memoryRequired')
emit('ARCHIVE_OUTPUT', app['archive']) if app.key?('archive')

emit('APP_JAR', resolve.call(backend['jar'])) if backend.key?('jar')
emit('APP_SERVER_NAME', backend['serverName']) if backend.key?('serverName')
emit('APP_INTERNAL_PORT', backend['internalPort']) if backend.key?('internalPort')
emit('APP_HOST_PORT', backend['hostPort']) if backend.key?('hostPort')
emit('APP_JAVA_IMAGE', backend['javaImage']) if backend.key?('javaImage')
emit('APP_TIMEZONE', backend['timezone']) if backend.key?('timezone')
emit('APP_JAVA_OPTS', backend['javaOpts']) if backend.key?('javaOpts')
emit('APP_APPLICATION_OPS', backend['applicationOps']) if backend.key?('applicationOps')
emit('APP_CONFIG_FILE', resolve.call(backend['configFile'])) if backend.key?('configFile')
emit('APP_CONFIG_TARGET', backend['configTarget']) if backend.key?('configTarget')

emit('WEB_DIR', resolve.call(web['dist'])) if web.key?('dist')
emit('WEB_ROUTE_PREFIX', web['routePrefix']) if web.key?('routePrefix')
emit('WEB_API_PREFIX', web['apiPrefix']) if web.key?('apiPrefix')
emit('WEB_API_PROXY_PASS', web['apiProxyPass']) if web.key?('apiProxyPass')
emit('WEB_ROOT_NAME', web['rootName']) if web.key?('rootName')

puts "APP_TAGS=()"
(app['tags'] || []).each { |tag| puts "APP_TAGS+=(#{shq(tag)})" }

puts "APP_ENVS=()"
(env['editable'] || {}).each { |k, v| puts "APP_ENVS+=(#{shq("#{k}=#{v}")})" }

puts "APP_FIXED_ENVS=()"
(env['fixed'] || {}).each { |k, v| puts "APP_FIXED_ENVS+=(#{shq("#{k}=#{v}")})" }

puts "FORM_OVERRIDES=()"
(form || {}).each do |k, meta|
  meta ||= {}
  parts = [
    k.to_s,
    (meta['labelZh'] || ''),
    (meta['labelEn'] || ''),
    (meta['type'] || ''),
    (meta['rule'] || ''),
    meta.key?('required') ? meta['required'].to_s : ''
  ]
  puts "FORM_OVERRIDES+=(#{shq(parts.join("\x1f"))})"
end
RUBY
    )"
}

guess_field_type() {
    local key="$1"
    if [[ "${key}" =~ (PASSWORD|PASSWD|PWD|TOKEN|SECRET|KEY)$ ]]; then
        printf 'password\n'
        return 0
    fi
    if [[ "${key}" =~ PORT ]]; then
        printf 'number\n'
        return 0
    fi
    printf 'text\n'
}

write_dynamic_env_yaml() {
    local item key
    if ((${#APP_ENVS[@]})); then
        for item in "${APP_ENVS[@]}"; do
            key="${item%%=*}"
            printf '      %s: "${%s}"\n' "${key}" "${key}"
        done
    fi
    if ((${#APP_FIXED_ENVS[@]})); then
        for item in "${APP_FIXED_ENVS[@]}"; do
            key="${item%%=*}"
            printf '      %s: "${%s}"\n' "${key}" "${key}"
        done
    fi
}

write_optional_config_mount() {
    if [[ -n "${APP_CONFIG_FILE}" ]]; then
        printf '      - ./config/%s:%s:ro\n' "$(basename "${APP_CONFIG_FILE}")" "${APP_CONFIG_TARGET}"
    fi
}

write_root_data_yml() {
    local app_dir="$1"
    local desc_zh="$2"
    local desc_en="$3"
    cat > "${app_dir}/data.yml" <<EOF
additionalProperties:
  name: ${APP_NAME}
  key: ${APP_KEY}
  type: ${APP_TYPE}
  tags:
$(for tag in "${APP_TAGS[@]}"; do printf '    - %s\n' "${tag}"; done)
  shortDescZh: ${APP_NAME}
  shortDescEn: ${APP_NAME}
  description:
    zh: "$(escape_yaml "${desc_zh}")"
    en: "$(escape_yaml "${desc_en}")"
  crossVersionUpdate: true
  limit: 1
  recommend: 100
  website: ""
  github: ""
  document: ""
  architectures:
    - amd64
    - arm64
  memoryRequired: ${APP_MEMORY_REQUIRED}
  gpuSupport: false
  batchInstallSupport: false
EOF
}

write_version_data_yml() {
    local version_dir="$1"
    cat > "${version_dir}/data.yml" <<EOF
additionalProperties:
  formFields:
    - type: text
      labelZh: Java 镜像
      labelEn: Java image
      required: true
      default: ${APP_JAVA_IMAGE}
      envKey: JAVA_IMAGES
    - type: text
      labelZh: 时区
      labelEn: Timezone
      required: true
      default: ${APP_TIMEZONE}
      envKey: TIMEZONE
    - type: number
      labelZh: 对外端口
      labelEn: Host port
      required: true
      default: ${APP_HOST_PORT}
      envKey: PANEL_APP_PORT_HTTP
      rule: port
EOF

    local item key value field_type label
    if ((${#APP_ENVS[@]})); then
        for item in "${APP_ENVS[@]}"; do
            key="${item%%=*}"
            value="${item#*=}"
            field_type="$(guess_field_type "${key}")"
            label="$(humanize_label "${key}")"
            local label_zh="${label}"
            local label_en="${label}"
            local rule=""
            local required="true"
            if find_form_override "${key}"; then
                if [[ -n "${CURRENT_FIELD_TYPE}" ]]; then
                    field_type="${CURRENT_FIELD_TYPE}"
                fi
                if [[ -n "${CURRENT_FIELD_LABEL_ZH}" ]]; then
                    label_zh="${CURRENT_FIELD_LABEL_ZH}"
                fi
                if [[ -n "${CURRENT_FIELD_LABEL_EN}" ]]; then
                    label_en="${CURRENT_FIELD_LABEL_EN}"
                fi
                if [[ -n "${CURRENT_FIELD_RULE}" ]]; then
                    rule="${CURRENT_FIELD_RULE}"
                fi
                required="$(resolve_required "${CURRENT_FIELD_REQUIRED}")"
            elif [[ "${field_type}" == "number" ]]; then
                rule="port"
            fi
            cat >> "${version_dir}/data.yml" <<EOF
    - type: ${field_type}
      labelZh: ${label_zh}
      labelEn: ${label_en}
      required: ${required}
      default: $(if [[ "${field_type}" == "number" ]]; then printf '%s' "${value}"; else printf '%s' "${value}"; fi)
      envKey: ${key}
$(if [[ -n "${rule}" ]]; then printf '      rule: %s\n' "${rule}"; fi)
EOF
        done
    fi
}

write_env_file() {
    local version_dir="$1"
    {
        printf 'JAVA_IMAGES=%s\n' "${APP_JAVA_IMAGE}"
        printf 'TIMEZONE=%s\n' "${APP_TIMEZONE}"
        printf 'PANEL_APP_PORT_HTTP=%s\n' "${APP_HOST_PORT}"
        if ((${#APP_ENVS[@]})); then
            for item in "${APP_ENVS[@]}"; do
                printf '%s\n' "${item}"
            done
        fi
        if ((${#APP_FIXED_ENVS[@]})); then
            for item in "${APP_FIXED_ENVS[@]}"; do
                printf '%s\n' "${item}"
            done
        fi
    } > "${version_dir}/.env"
}

write_readme() {
    local app_dir="$1"
    cat > "${app_dir}/README.md" <<EOF
# ${APP_NAME}

This package was generated by \`scripts/generate_1panel_java_app.sh\`.

Install steps:

1. Copy this app directory into the 1Panel local app directory.
2. Open 1Panel App Store.
3. Click "Sync Local App".
4. Install ${APP_NAME}.

Container behavior:

- Jar file will run as \`/app/${APP_SERVER_NAME}.jar\`
- Host port form field defaults to \`${APP_HOST_PORT}\`
- Internal service port defaults to \`${APP_INTERNAL_PORT}\`
- Startup script is generated automatically by this scaffold
EOF
}

write_start_java_script() {
    local version_dir="$1"
    cat > "${version_dir}/startJava.sh" <<'EOF'
#!/usr/bin/env sh

set -eu

if [ -n "${TIMEZONE:-}" ]; then
  TIMEZONE_FILE="/usr/share/zoneinfo/${TIMEZONE}"
  if [ -f "${TIMEZONE_FILE}" ]; then
    echo "Replacing /etc/localtime with ${TIMEZONE_FILE}"
    ln -sf "${TIMEZONE_FILE}" /etc/localtime
  else
    echo "Timezone file for '${TIMEZONE}' does not exist, skipping timezone replacement"
  fi
else
  echo "No timezone specified, skipping timezone replacement"
fi

shutdown_handler() {
  echo "SIGTERM received, shutting down Java process..."
  if [ -n "${JAVA_PID:-}" ]; then
    kill -15 "${JAVA_PID}" 2>/dev/null || true
    wait "${JAVA_PID}" 2>/dev/null || true
  fi
  echo "Java process terminated."
  exit 0
}

trap shutdown_handler TERM INT

echo "Starting Java application..."
java -server ${JAVA_OPS:-} -jar "/app/${SERVER_NAME}.jar" ${APPLICATION_OPS:-} &
JAVA_PID=$!

wait "${JAVA_PID}"
EOF
    chmod +x "${version_dir}/startJava.sh"
}

write_backend_compose() {
    local version_dir="$1"
    local api_service_name="${APP_KEY}"
    if [[ "${api_service_name}" != *-api ]]; then
        api_service_name="${api_service_name}-api"
    fi
    cat > "${version_dir}/docker-compose.yml" <<EOF
services:
  ${api_service_name}:
    image: \${JAVA_IMAGES}
    container_name: ${api_service_name}
    environment:
      PROJECT_NAME: ${APP_KEY}
      SERVER_PORT: "${APP_INTERNAL_PORT}"
      JAVA_OPS: $(quote_compose "${APP_JAVA_OPTS}")
      SERVER_NAME: ${APP_SERVER_NAME}
      APPLICATION_OPS: $(quote_compose "${APP_APPLICATION_OPS}")
      TIMEZONE: \${TIMEZONE}
$(write_dynamic_env_yaml)
    command: /startJava.sh
    ports:
      - \${PANEL_APP_PORT_HTTP}:${APP_INTERNAL_PORT}
    volumes:
      - ./data:/app
      - ./logs:/logs
      - ./startJava.sh:/startJava.sh:ro
$(write_optional_config_mount)
    restart: always

networks:
  default:
    name: 1panel-network
    external: true
EOF
}

write_web_compose() {
    local version_dir="$1"
    local api_service_name="${APP_KEY}"
    local web_service_name="${APP_KEY}"
    if [[ "${api_service_name}" != *-api ]]; then
        api_service_name="${api_service_name}-api"
    fi
    if [[ "${web_service_name}" != *-web ]]; then
        web_service_name="${web_service_name}-web"
    fi
    cat > "${version_dir}/docker-compose.yml" <<EOF
services:
  ${api_service_name}:
    image: \${JAVA_IMAGES}
    container_name: ${api_service_name}
    environment:
      PROJECT_NAME: ${APP_KEY}
      SERVER_PORT: "${APP_INTERNAL_PORT}"
      JAVA_OPS: $(quote_compose "${APP_JAVA_OPTS}")
      SERVER_NAME: ${APP_SERVER_NAME}
      APPLICATION_OPS: $(quote_compose "${APP_APPLICATION_OPS}")
      TIMEZONE: \${TIMEZONE}
$(write_dynamic_env_yaml)
    command: /startJava.sh
    volumes:
      - ./data:/app
      - ./logs:/logs
      - ./startJava.sh:/startJava.sh:ro
$(write_optional_config_mount)
    restart: always

  ${web_service_name}:
    image: nginx:1.27.1
    container_name: ${web_service_name}
    depends_on:
      - ${api_service_name}
    ports:
      - \${PANEL_APP_PORT_HTTP}:80
    volumes:
      - ./web:/usr/share/nginx/${WEB_ROOT_NAME}:ro
      - ./config/nginx/default.conf:/etc/nginx/conf.d/default.conf:ro
    restart: always

networks:
  default:
    name: 1panel-network
    external: true
EOF
}

write_nginx_conf() {
    local version_dir="$1"
    local api_service_name="${APP_KEY}"
    if [[ "${api_service_name}" != *-api ]]; then
        api_service_name="${api_service_name}-api"
    fi
    local route_prefix="${WEB_ROUTE_PREFIX}"
    local api_prefix="${WEB_API_PREFIX}"
    local proxy_pass="${WEB_API_PROXY_PASS}"

    if [[ -z "${proxy_pass}" ]]; then
        proxy_pass="http://${api_service_name}:${APP_INTERNAL_PORT}/"
    fi

    mkdir -p "${version_dir}/config/nginx"
    cat > "${version_dir}/config/nginx/default.conf" <<EOF
server {
    listen 80;
    server_name _;
    client_max_body_size 100m;

    location ${route_prefix} {
        alias /usr/share/nginx/${WEB_ROOT_NAME};
        index index.html;
        try_files \$uri \$uri/ ${route_prefix}/index.html;
    }

    location ^~${api_prefix} {
        proxy_pass ${proxy_pass};
        proxy_connect_timeout 60s;
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$http_host;
    }

    location / {
        return 404;
    }
}
EOF
}

create_archive() {
    local app_dir="$1"
    local archive_path="${APP_OUTPUT_ROOT}/${APP_KEY}-${APP_VERSION}.tar.gz"
    LC_ALL=C tar -C "${APP_OUTPUT_ROOT}" -czf "${archive_path}" "$(basename "${app_dir}")"
    printf 'created archive %s\n' "${archive_path}"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --spec) SPEC_FILE="$2"; shift 2 ;;
        --jar) APP_JAR="$2"; shift 2 ;;
        --app-key) APP_KEY="$2"; shift 2 ;;
        --app-name) APP_NAME="$2"; shift 2 ;;
        --version) APP_VERSION="$2"; shift 2 ;;
        --output-root) APP_OUTPUT_ROOT="$2"; shift 2 ;;
        --server-name) APP_SERVER_NAME="$2"; shift 2 ;;
        --internal-port) APP_INTERNAL_PORT="$2"; shift 2 ;;
        --host-port) APP_HOST_PORT="$2"; shift 2 ;;
        --java-image) APP_JAVA_IMAGE="$2"; shift 2 ;;
        --timezone) APP_TIMEZONE="$2"; shift 2 ;;
        --java-opts) APP_JAVA_OPTS="$2"; shift 2 ;;
        --application-ops) APP_APPLICATION_OPS="$2"; shift 2 ;;
        --config-file) APP_CONFIG_FILE="$2"; shift 2 ;;
        --config-target) APP_CONFIG_TARGET="$2"; shift 2 ;;
        --logo) APP_LOGO="$2"; shift 2 ;;
        --desc-zh) APP_DESC_ZH="$2"; shift 2 ;;
        --desc-en) APP_DESC_EN="$2"; shift 2 ;;
        --tag) APP_TAGS+=("$2"); shift 2 ;;
        --env) APP_ENVS+=("$2"); shift 2 ;;
        --fixed-env) APP_FIXED_ENVS+=("$2"); shift 2 ;;
        --memory-required) APP_MEMORY_REQUIRED="$2"; shift 2 ;;
        --web-dir) WEB_DIR="$2"; shift 2 ;;
        --route-prefix) WEB_ROUTE_PREFIX="$2"; shift 2 ;;
        --api-prefix) WEB_API_PREFIX="$2"; shift 2 ;;
        --api-proxy-pass) WEB_API_PROXY_PASS="$2"; shift 2 ;;
        --web-root-name) WEB_ROOT_NAME="$2"; shift 2 ;;
        --archive) ARCHIVE_OUTPUT="true"; shift ;;
        -h|--help) usage; exit 0 ;;
        *) die "unknown argument: $1" ;;
    esac
done

if [[ -n "${SPEC_FILE}" ]]; then
    load_spec_file "${SPEC_FILE}"
fi

[[ -n "${APP_KEY}" ]] || die "--app-key or --spec is required"
[[ -n "${APP_NAME}" ]] || die "--app-name or --spec is required"
[[ -n "${APP_JAR}" ]] || die "--jar or --spec is required"
require_file "${APP_JAR}"
require_file "${PLACEHOLDER_LOGO}"
if [[ -n "${APP_CONFIG_FILE}" ]]; then
    require_file "${APP_CONFIG_FILE}"
fi
if [[ -n "${WEB_DIR}" ]]; then
    [[ -d "${WEB_DIR}" ]] || die "web dir not found: ${WEB_DIR}"
fi

if [[ -z "${APP_SERVER_NAME}" ]]; then
    APP_SERVER_NAME="${APP_KEY##*/}"
fi
if [[ -z "${APP_LOGO}" ]]; then
    APP_LOGO="${PLACEHOLDER_LOGO}"
fi
if [[ -z "${APP_DESC_ZH}" ]]; then
    APP_DESC_ZH="安装 ${APP_NAME} 本地应用包。"
fi
if [[ -z "${APP_DESC_EN}" ]]; then
    APP_DESC_EN="Install the ${APP_NAME} local app package."
fi
if [[ ${#APP_TAGS[@]} -eq 0 ]]; then
    APP_TAGS=("custom")
fi
if [[ -z "${APP_CONFIG_TARGET}" && -n "${APP_CONFIG_FILE}" ]]; then
    APP_CONFIG_TARGET="/app/$(basename "${APP_CONFIG_FILE}")"
fi
if [[ -n "${WEB_DIR}" && -z "${WEB_ROUTE_PREFIX}" ]]; then
    WEB_ROUTE_PREFIX="/${APP_KEY#package-}"
fi
if [[ -n "${WEB_DIR}" && -z "${WEB_ROOT_NAME}" ]]; then
    WEB_ROOT_NAME="${APP_KEY#package-}-ui"
fi

APP_DIR="${APP_OUTPUT_ROOT}/${APP_KEY}"
VERSION_DIR="${APP_DIR}/${APP_VERSION}"

rm -rf "${APP_DIR}"
mkdir -p "${VERSION_DIR}/data" "${VERSION_DIR}/logs"

cp "${APP_LOGO}" "${APP_DIR}/logo.png"
write_start_java_script "${VERSION_DIR}"
cp "${APP_JAR}" "${VERSION_DIR}/data/${APP_SERVER_NAME}.jar"
if [[ -n "${APP_CONFIG_FILE}" ]]; then
    mkdir -p "${VERSION_DIR}/config"
    cp "${APP_CONFIG_FILE}" "${VERSION_DIR}/config/$(basename "${APP_CONFIG_FILE}")"
fi
if [[ -n "${WEB_DIR}" ]]; then
    mkdir -p "${VERSION_DIR}/web"
    cp -R "${WEB_DIR}/." "${VERSION_DIR}/web/"
fi

write_root_data_yml "${APP_DIR}" "${APP_DESC_ZH}" "${APP_DESC_EN}"
write_version_data_yml "${VERSION_DIR}"
write_env_file "${VERSION_DIR}"
write_readme "${APP_DIR}"

if [[ -n "${WEB_DIR}" ]]; then
    write_web_compose "${VERSION_DIR}"
    write_nginx_conf "${VERSION_DIR}"
else
    write_backend_compose "${VERSION_DIR}"
fi

printf 'generated local app package in %s\n' "${APP_DIR}"
if [[ "${ARCHIVE_OUTPUT}" == "true" ]]; then
    create_archive "${APP_DIR}"
fi
