package gardeninstaller

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"
)

// configTemplateData holds the data for config templates.
type configTemplateData struct {
	BaseDir          string
	ListenNetwork    string
	ListenAddress    string
	AllowHostAccess  bool
	DestroyOnStart   bool
	DebugListenAddr  string
	DefaultGraceTime string
	GraphCleanupMB   string
}

// generateConfigs creates the Garden config files on the target.
func (i *Installer) generateConfigs() error {
	data := configTemplateData{
		BaseDir:          i.cfg.BaseDir,
		ListenNetwork:    i.cfg.ListenNetwork,
		ListenAddress:    i.cfg.ListenAddress,
		AllowHostAccess:  i.cfg.AllowHostAccess,
		DestroyOnStart:   i.cfg.DestroyOnStart,
		DebugListenAddr:  "127.0.0.1:17013",
		DefaultGraceTime: "0",
		GraphCleanupMB:   "0",
	}

	// Generate config.ini
	configPath := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "config.ini")
	configContent, err := renderTemplate(configIniTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render config.ini: %w", err)
	}
	if err := i.driver.WriteFile(configPath, configContent, 0644); err != nil {
		return fmt.Errorf("failed to write config.ini: %w", err)
	}
	i.log("Generated config: %s", configPath)

	// Generate garden_ctl
	ctlPath := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "garden_ctl")
	ctlContent, err := renderTemplate(gardenCtlTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render garden_ctl: %w", err)
	}
	if err := i.driver.WriteFile(ctlPath, ctlContent, 0755); err != nil {
		return fmt.Errorf("failed to write garden_ctl: %w", err)
	}
	i.log("Generated script: %s", ctlPath)

	// Generate grootfs config
	grootfsConfigPath := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "grootfs_config.yml")
	grootfsContent, err := renderTemplate(grootfsConfigTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render grootfs_config.yml: %w", err)
	}
	if err := i.driver.WriteFile(grootfsConfigPath, grootfsContent, 0644); err != nil {
		return fmt.Errorf("failed to write grootfs_config.yml: %w", err)
	}
	i.log("Generated config: %s", grootfsConfigPath)

	// Generate privileged grootfs config
	privGrootfsConfigPath := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "privileged_grootfs_config.yml")
	privGrootfsContent, err := renderTemplate(privilegedGrootfsConfigTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render privileged_grootfs_config.yml: %w", err)
	}
	if err := i.driver.WriteFile(privGrootfsConfigPath, privGrootfsContent, 0644); err != nil {
		return fmt.Errorf("failed to write privileged_grootfs_config.yml: %w", err)
	}
	i.log("Generated config: %s", privGrootfsConfigPath)

	// Generate envs script (needed by garden_ctl and other scripts)
	envsPath := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "envs")
	envsContent, err := renderTemplate(envsTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render envs: %w", err)
	}
	if err := i.driver.WriteFile(envsPath, envsContent, 0755); err != nil {
		return fmt.Errorf("failed to write envs: %w", err)
	}
	i.log("Generated script: %s", envsPath)

	// Generate grootfs-utils script (needed by overlay-xfs-setup)
	grootfsUtilsPath := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "grootfs-utils")
	grootfsUtilsContent, err := renderTemplate(grootfsUtilsTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render grootfs-utils: %w", err)
	}
	if err := i.driver.WriteFile(grootfsUtilsPath, grootfsUtilsContent, 0755); err != nil {
		return fmt.Errorf("failed to write grootfs-utils: %w", err)
	}
	i.log("Generated script: %s", grootfsUtilsPath)

	return nil
}

// renderTemplate renders a template with the given data and returns the result as bytes.
func renderTemplate(tmplContent string, data interface{}) ([]byte, error) {
	tmpl, err := template.New("config").Parse(tmplContent)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// configIniTemplate is the Garden configuration file template.
// Based on the garden job spec from garden-runc-release.
// Note: bind-ip and bind-port enable TCP listening (in addition to default unix socket).
const configIniTemplate = `[server]
; binaries
  iptables-bin = {{.BaseDir}}/packages/iptables/sbin/iptables
  iptables-restore-bin = {{.BaseDir}}/packages/iptables/sbin/iptables-restore
  init-bin = {{.BaseDir}}/data/garden/bin/init
  dadoo-bin = {{.BaseDir}}/packages/guardian/bin/dadoo
  nstar-bin = {{.BaseDir}}/packages/guardian/bin/nstar
  tar-bin = {{.BaseDir}}/packages/tar/tar

; containers
  default-grace-time = {{.DefaultGraceTime}}
  destroy-containers-on-startup = true

; image and rootfs
  image-plugin = {{.BaseDir}}/packages/grootfs/bin/grootfs
  image-plugin-extra-arg = --config
  image-plugin-extra-arg = {{.BaseDir}}/jobs/garden/config/grootfs_config.yml
  privileged-image-plugin = {{.BaseDir}}/packages/grootfs/bin/grootfs
  privileged-image-plugin-extra-arg = --config
  privileged-image-plugin-extra-arg = {{.BaseDir}}/jobs/garden/config/privileged_grootfs_config.yml

; network
  allow-host-access = true
  network-plugin = /bin/true

; properties
  properties-path = {{.BaseDir}}/data/garden/props.json
  port-pool-properties-path = {{.BaseDir}}/data/garden/port-pool-props.json

; server
  bind-ip = 0.0.0.0
  bind-port = 7777
  log-level = info
  skip-setup = true
  depot = {{.BaseDir}}/data/garden/depot
  runtime-plugin = {{.BaseDir}}/packages/runc/bin/runc
`

// gardenCtlTemplate is the Garden control script template.
const gardenCtlTemplate = `#!/bin/bash

set -e

RUN_DIR="{{.BaseDir}}/sys/run/garden"
LOG_DIR="{{.BaseDir}}/sys/log/garden"
PIDFILE="${RUN_DIR}/garden.pid"
RUNTIME_BIN_DIR="{{.BaseDir}}/data/garden/bin"
DEPOT_DIR="{{.BaseDir}}/data/garden/depot"
# gdn has hardcoded /var/run/gdn/depot for "peas" (sidecar containers)
PEA_DEPOT_DIR="/var/run/gdn/depot"

GARDEN_BIN="{{.BaseDir}}/packages/guardian/bin/gdn"
GARDEN_CONFIG="{{.BaseDir}}/jobs/garden/config/config.ini"

export PATH="{{.BaseDir}}/packages/runc/bin:{{.BaseDir}}/packages/grootfs/bin:{{.BaseDir}}/packages/garden-idmapper/bin:$PATH"
export TMPDIR="{{.BaseDir}}/data/tmp"

# Get maximus for uid/gid mapping
MAXIMUS=$(maximus)

# Ensure directories exist
mkdir -p "$RUN_DIR" "$LOG_DIR" "$TMPDIR" "$RUNTIME_BIN_DIR" "$DEPOT_DIR" "$PEA_DEPOT_DIR"

case "$1" in
  start)
    echo "Starting Garden..."
    
    if [ -f "$PIDFILE" ]; then
      pid=$(cat "$PIDFILE")
      if kill -0 "$pid" 2>/dev/null; then
        echo "Garden is already running (pid $pid)"
        exit 0
      fi
      rm -f "$PIDFILE"
    fi

    # Copy init binary (cannot overwrite while running)
    rm -f "$RUNTIME_BIN_DIR/init"
    cp {{.BaseDir}}/packages/guardian/bin/init "$RUNTIME_BIN_DIR/init"

    # Run gdn setup first (prepares cgroups, etc.)
    echo "Running gdn setup..."
    "$GARDEN_BIN" setup \
      >> "$LOG_DIR/garden.stdout.log" \
      2>> "$LOG_DIR/garden.stderr.log"

    # Start gdn server in background with uid/gid mapping
    # Note: --config is a global flag and must come BEFORE the 'server' subcommand
    "$GARDEN_BIN" --config "$GARDEN_CONFIG" server \
      --uid-map-start=1 \
      --uid-map-length=$((MAXIMUS-1)) \
      --gid-map-start=1 \
      --gid-map-length=$((MAXIMUS-1)) \
      >> "$LOG_DIR/garden.stdout.log" \
      2>> "$LOG_DIR/garden.stderr.log" &
    
    echo $! > "$PIDFILE"
    echo "Garden started (pid $(cat $PIDFILE))"
    
    # Wait a moment and verify it's running
    sleep 2
    if ! kill -0 "$(cat $PIDFILE)" 2>/dev/null; then
      echo "ERROR: Garden failed to start. Check logs at $LOG_DIR"
      cat "$LOG_DIR/garden.stderr.log" | tail -50
      exit 1
    fi
    ;;

  stop)
    echo "Stopping Garden..."
    if [ -f "$PIDFILE" ]; then
      pid=$(cat "$PIDFILE")
      if kill -0 "$pid" 2>/dev/null; then
        kill "$pid"
        # Wait for process to exit
        for i in $(seq 1 30); do
          if ! kill -0 "$pid" 2>/dev/null; then
            break
          fi
          sleep 1
        done
        if kill -0 "$pid" 2>/dev/null; then
          echo "Garden didn't stop gracefully, forcing..."
          kill -9 "$pid" 2>/dev/null || true
        fi
      fi
      rm -f "$PIDFILE"
    fi
    echo "Garden stopped"
    ;;

  status)
    if [ -f "$PIDFILE" ]; then
      pid=$(cat "$PIDFILE")
      if kill -0 "$pid" 2>/dev/null; then
        echo "Garden is running (pid $pid)"
        exit 0
      fi
    fi
    echo "Garden is not running"
    exit 1
    ;;

  *)
    echo "Usage: $0 {start|stop|status}"
    exit 1
    ;;
esac
`

// grootfsConfigTemplate is the GrootFS configuration for unprivileged containers.
// Note: We don't specify 'driver' to let grootfs auto-detect based on store path filesystem.
const grootfsConfigTemplate = `store: {{.BaseDir}}/data/grootfs/store/unprivileged
tardis_bin: {{.BaseDir}}/packages/grootfs/bin/tardis
newuidmap_bin: {{.BaseDir}}/packages/grootfs/bin/newuidmap
newgidmap_bin: {{.BaseDir}}/packages/grootfs/bin/newgidmap
log_level: info
create:
  with_clean: true
  without_mount: false
  skip_layer_validation: true
`

// privilegedGrootfsConfigTemplate is the GrootFS configuration for privileged containers.
const privilegedGrootfsConfigTemplate = `store: {{.BaseDir}}/data/grootfs/store/privileged
tardis_bin: {{.BaseDir}}/packages/grootfs/bin/tardis
log_level: info
create:
  with_clean: true
  without_mount: false
  skip_layer_validation: true
`

// grootfsUtilsTemplate is the grootfs-utils script used by overlay-xfs-setup.
// This is a rendered version of the ERB template with default values.
const grootfsUtilsTemplate = `#!/bin/bash

export store_mountpoint="{{.BaseDir}}/data"

invoke_thresholder() {
  log "running thresholder"
  # Thresholder with default values (reserved=0, routine_gc=false, cleanup_threshold=0)
  {{.BaseDir}}/packages/thresholder/bin/thresholder "0" "false" "$DATA_DIR" "$GARDEN_CONFIG_DIR/grootfs_config.yml" "0" "0"
  {{.BaseDir}}/packages/thresholder/bin/thresholder "0" "false" "$DATA_DIR" "$GARDEN_CONFIG_DIR/privileged_grootfs_config.yml" "0" "0"
  log "done"
}

unprivileged_root_mapping() {
  maximus_uid=$({{.BaseDir}}/packages/garden-idmapper/bin/maximus)
  echo -n "0:${maximus_uid}:1"
}

unprivileged_range_mapping() {
  maximus_uid=$({{.BaseDir}}/packages/garden-idmapper/bin/maximus)
  range="1:1:$((maximus_uid-1))"
  echo -n $range
}

log() {
  local msg
  local time

  msg=$1
  time=$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")

  echo "$time - $msg"
}
`

// envsTemplate is the envs script that sets environment variables for garden scripts.
// Based on templates/bin/envs.erb from garden-runc-release.
const envsTemplate = `#!/bin/bash

export BASE_PATH=$(dirname $0)
export RUN_DIR="{{.BaseDir}}/sys/run/garden"
export LOG_DIR="{{.BaseDir}}/sys/log/garden"
export PIDFILE="${RUN_DIR}/garden.pid"
export DATA_DIR="{{.BaseDir}}/data"
export GARDEN_DATA_DIR="{{.BaseDir}}/data/garden"
export GARDEN_CONFIG_DIR="{{.BaseDir}}/jobs/garden/config"
export GARDEN_CERTS_DIR="{{.BaseDir}}/jobs/garden/certs"
export GARDEN_CONFIG_PATH="${GARDEN_CONFIG_DIR}/config.ini"
export DEPOT_PATH="${GARDEN_DATA_DIR}/depot"
export RUNTIME_BIN_DIR="${GARDEN_DATA_DIR}/bin"
export TMPDIR="{{.BaseDir}}/data/tmp"

export PATH="{{.BaseDir}}/packages/guardian/bin:${PATH}"
export PATH="{{.BaseDir}}/packages/iptables/sbin:${PATH}"
export PATH="{{.BaseDir}}/packages/garden-idmapper/bin:${PATH}"
export PATH="{{.BaseDir}}/packages/grootfs/bin:${PATH}"
export PATH="{{.BaseDir}}/packages/xfs-progs/sbin:${PATH}"
export PATH="{{.BaseDir}}/packages/thresholder/bin:${PATH}"
export PATH="{{.BaseDir}}/packages/greenskeeper/bin:${PATH}"
export PATH="{{.BaseDir}}/packages/runc/bin:${PATH}"
export PATH="{{.BaseDir}}/jobs/garden/bin:${PATH}"

export MAXIMUS=$(maximus)

# Check if running under systemd
if [ -d /run/systemd/system ]; then
  export IS_RUNNING_SYSTEMD=true
fi
`
