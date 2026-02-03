#!/bin/bash
# Script to cross-compile the bosh-agent and deploy it to a Noble VM for debugging
# Uses os-conf user_add for SSH access to ensure it works even when agent is broken
#
# Garden is installed automatically by the test suite using the gardeninstaller
# package when GARDEN_RELEASE_TARBALL is set.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-bosh-agent-integration-firewall-noble}"
INSTANCE_GROUP="${INSTANCE_GROUP:-agent-test}"
INSTANCE_ID="${INSTANCE_ID:-0}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Check if bosh CLI is available and configured
check_bosh() {
    if ! command -v bosh &> /dev/null; then
        log_error "bosh CLI not found. Please install it or source your bosh environment."
        exit 1
    fi
    
    if ! bosh env &> /dev/null; then
        log_error "BOSH not configured. Please source your bosh environment file."
        log_info "Example: source ~/workspace/noble-concourse-nested-cpi-validation/bosh.env"
        exit 1
    fi
    
    log_info "BOSH environment: $(bosh env --json 2>/dev/null | jq -r '.Tables[0].Rows[0].name // "unknown"')"
}

# Create deployment (Garden is installed by the test suite via gardeninstaller)
create_deployment() {
    log_info "Creating deployment ${DEPLOYMENT_NAME}..."
    
    # Get SSH public key for user_add
    local ssh_pubkey
    if [[ -f "${SCRIPT_DIR}/debug-ssh-key.pub" ]]; then
        ssh_pubkey=$(cat "${SCRIPT_DIR}/debug-ssh-key.pub")
        log_info "Using debug SSH key from repo"
    elif [[ -f "$HOME/.ssh/id_rsa.pub" ]]; then
        ssh_pubkey=$(cat "$HOME/.ssh/id_rsa.pub")
    elif [[ -f "$HOME/.ssh/id_ed25519.pub" ]]; then
        ssh_pubkey=$(cat "$HOME/.ssh/id_ed25519.pub")
    else
        log_error "No SSH public key found. Create one with: ssh-keygen -t ed25519"
        exit 1
    fi
    log_info "Using SSH key: ${ssh_pubkey:0:50}..."
    
    # Create deployment manifest without garden-runc
    # Garden is installed by the test suite using gardeninstaller
    cat > /tmp/agent-deployment.yml <<EOF
name: ${DEPLOYMENT_NAME}
releases:
  - name: os-conf
    version: latest
stemcells:
  - alias: stemcell
    os: ubuntu-noble
    version: latest
instance_groups:
  - name: ${INSTANCE_GROUP}
    azs: [z1]
    instances: 1
    vm_type: garden-test
    stemcell: stemcell
    networks:
      - name: default
    jobs:
      - name: user_add
        release: os-conf
        properties:
          users:
            - name: agent_test_user
              public_key: "${ssh_pubkey}"
      - name: pre-start-script
        release: os-conf
        properties:
          script: |
            #!/bin/bash
            # Install LXD/incus agent for incus exec access (debugging fallback)
            if [ -e /dev/virtio-ports/org.linuxcontainers.lxd ]; then
              mount -t 9p config /mnt || true
              if [ -f /mnt/install.sh ]; then
                cd /mnt
                ./install.sh
                cd /
                umount /mnt
                systemctl start lxd-agent || true
              fi
            fi
    env:
      bosh:
        ipv6:
          enable: true
update:
  canaries: 0
  canary_watch_time: 60000
  max_in_flight: 1
  update_watch_time: 60000
EOF

    log_info "Deploying..."
    bosh -n -d "$DEPLOYMENT_NAME" deploy /tmp/agent-deployment.yml
    
    local vm_ip
    vm_ip=$(get_vm_ip)
    log_info "Deployment complete! VM IP: $vm_ip"
    
    # Install nftables CLI
    log_info "Installing nftables CLI..."
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "sudo apt-get update && sudo apt-get install -y nftables" 2>/dev/null || true
    
    log_info ""
    log_info "Next steps:"
    log_info "  1. Compile garden-runc: bin/compile-garden-release.sh"
    log_info "  2. Run tests: GARDEN_RELEASE_TARBALL=./compiled-releases/garden-runc-*.tgz $0 test-garden"
}

# Delete deployment
delete_deployment() {
    log_info "Deleting deployment ${DEPLOYMENT_NAME}..."
    bosh -n -d "$DEPLOYMENT_NAME" delete-deployment --force || true
    log_info "Deployment deleted"
}

# Show help
show_help() {
    echo "Usage: $0 [create|delete|build|deploy|start|stop|logs|nft|garden|test-garden|ssh]"
    echo ""
    echo "Deployment management:"
    echo "  create       - Create a new deployment"
    echo "  delete       - Delete the deployment"
    echo ""
    echo "Agent commands:"
    echo "  build        - Only build the agent"
    echo "  deploy       - Build, deploy, configure, and start the agent (default)"
    echo "  start        - Start the agent service"
    echo "  stop         - Stop the agent service"
    echo "  logs         - Show recent agent logs"
    echo ""
    echo "Testing:"
    echo "  nft          - Check nftables status"
    echo "  garden       - Check Garden status"
    echo "  test-garden  - Run Garden container firewall tests"
    echo "  ssh          - Open SSH session to the VM"
    echo ""
    echo "Environment variables:"
    echo "  DEPLOYMENT_NAME         - BOSH deployment name (default: bosh-agent-integration-firewall-noble)"
    echo "  INSTANCE_GROUP          - Instance group name (default: agent-test)"
    echo "  INSTANCE_ID             - Instance ID (default: 0)"
    echo "  GARDEN_RELEASE_TARBALL  - Path to compiled garden-runc release tarball"
    echo "  STEMCELL_IMAGE          - Specific stemcell image to test (default: tests both Noble and Jammy)"
    echo ""
    echo "To compile a Garden release tarball:"
    echo "  bin/compile-garden-release.sh"
}

# Check Garden status
check_garden() {
    local vm_ip="${1:-$(get_vm_ip)}"
    
    log_info "Checking Garden status..."
    
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "
        echo '=== Garden process ==='
        ps aux | grep -E '(gdn|guardian)' | grep -v grep || echo 'Not running'
        echo ''
        echo '=== Garden ping ==='
        curl -s http://localhost:7777/ping && echo ' - Garden is responding' || echo 'Garden ping failed'
        echo ''
        echo '=== Garden listening ==='
        ss -tlnp | grep 7777 || echo 'Not listening on 7777'
        echo ''
        echo '=== Recent logs ==='
        sudo tail -20 /var/vcap/sys/log/garden/garden.stderr.log 2>/dev/null || echo 'No logs found'
    " 2>/dev/null
}

# Run Garden firewall tests
run_garden_tests() {
    local vm_ip
    vm_ip=$(get_vm_ip)
    
    log_info "Setting up environment for Garden tests..."
    
    # Build the agent binary for container tests
    # Use CGO_ENABLED=0 to create a static binary that works in containers
    log_info "Building agent binary for container tests (static)..."
    cd "$SCRIPT_DIR"
    CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o bosh-agent-linux-amd64 ./main
    
    # Build nft-dump utility for inspecting nftables without nft CLI
    log_info "Building nft-dump utility for container tests..."
    CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o nft-dump-linux-amd64 ./integration/nftdump
    
    # Determine SSH key path for agent VM
    local ssh_key_path
    if [[ -f "${SCRIPT_DIR}/debug-ssh-key" ]]; then
        ssh_key_path="${SCRIPT_DIR}/debug-ssh-key"
    elif [[ -f "$HOME/.ssh/id_rsa" ]]; then
        ssh_key_path="$HOME/.ssh/id_rsa"
    elif [[ -f "$HOME/.ssh/id_ed25519" ]]; then
        ssh_key_path="$HOME/.ssh/id_ed25519"
    else
        log_error "No SSH private key found"
        exit 1
    fi
    
    # Set up jumpbox for Garden client SSH tunnel
    # Use BOSH director as jumpbox (standard setup for BOSH deployments)
    local jumpbox_ip jumpbox_user jumpbox_key
    
    if [[ -n "${JUMPBOX_IP:-}" && -n "${JUMPBOX_KEY_PATH:-}" ]]; then
        # Use explicitly provided jumpbox settings
        jumpbox_ip="${JUMPBOX_IP}"
        jumpbox_user="${JUMPBOX_USERNAME:-jumpbox}"
        jumpbox_key="${JUMPBOX_KEY_PATH}"
    elif [[ -n "${BOSH_ENVIRONMENT:-}" ]]; then
        # Extract director IP from BOSH_ENVIRONMENT (https://IP:port)
        jumpbox_ip=$(echo "$BOSH_ENVIRONMENT" | sed -E 's|https?://([^:]+):.*|\1|')
        jumpbox_user="jumpbox"
        # Look for jumpbox key in common locations
        local pipeline_dir="$HOME/workspace/noble-concourse-nested-cpi-validation"
        if [[ -f "${pipeline_dir}/jumpbox-private-key.pem" ]]; then
            jumpbox_key="${pipeline_dir}/jumpbox-private-key.pem"
        elif [[ -f "${SCRIPT_DIR}/jumpbox-private-key.pem" ]]; then
            jumpbox_key="${SCRIPT_DIR}/jumpbox-private-key.pem"
        else
            log_error "Jumpbox key not found. Set JUMPBOX_KEY_PATH or place jumpbox-private-key.pem in workspace"
            exit 1
        fi
    else
        log_error "Cannot determine jumpbox settings. Set BOSH_ENVIRONMENT or JUMPBOX_* variables"
        exit 1
    fi
    
    export JUMPBOX_IP="${jumpbox_ip}"
    export JUMPBOX_USERNAME="${jumpbox_user}"
    export JUMPBOX_KEY_PATH="${jumpbox_key}"
    log_info "Using jumpbox: ${JUMPBOX_USERNAME}@${JUMPBOX_IP}"
    
    # Create ssh-config for the main integration test environment
    log_info "Creating ssh-config for integration tests..."
    cat > "${SCRIPT_DIR}/integration/ssh-config" <<EOF
Host agent_vm
  User agent_test_user
  HostName ${vm_ip}
  Port 22
  IdentityFile ${ssh_key_path}
  ProxyJump ${JUMPBOX_IP}
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null

Host jumpbox
  User ${JUMPBOX_USERNAME}
  HostName ${JUMPBOX_IP}
  Port 22
  IdentityFile ${JUMPBOX_KEY_PATH}
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
EOF

    # Set up environment variables
    export AGENT_IP="$vm_ip"
    export AGENT_KEY_PATH="$ssh_key_path"
    export AGENT_USER="agent_test_user"
    
    # Set GARDEN_ADDRESS if Garden is already running, otherwise the test suite will install it
    if curl -sf --connect-timeout 2 "http://${vm_ip}:7777/ping" 2>/dev/null; then
        export GARDEN_ADDRESS="${vm_ip}:7777"
        log_info "Garden already running at ${GARDEN_ADDRESS}"
    else
        log_info "Garden not running - test suite will install it if GARDEN_RELEASE_TARBALL is set"
        if [[ -z "${GARDEN_RELEASE_TARBALL:-}" ]]; then
            log_warn "GARDEN_RELEASE_TARBALL not set - tests may fail"
            log_info "Create compiled release with: bin/compile-garden-release.sh"
        else
            # Convert to absolute path since we'll cd to integration/garden
            export GARDEN_RELEASE_TARBALL="$(cd "$(dirname "$GARDEN_RELEASE_TARBALL")" && pwd)/$(basename "$GARDEN_RELEASE_TARBALL")"
        fi
    fi
    
    log_info "Running Garden container firewall tests..."
    log_info "  AGENT_IP=$AGENT_IP"
    log_info "  AGENT_KEY_PATH=$AGENT_KEY_PATH"
    log_info "  JUMPBOX_IP=${JUMPBOX_IP}"
    log_info "  JUMPBOX_KEY_PATH=${JUMPBOX_KEY_PATH}"
    [[ -n "${GARDEN_ADDRESS:-}" ]] && log_info "  GARDEN_ADDRESS=$GARDEN_ADDRESS"
    [[ -n "${GARDEN_RELEASE_TARBALL:-}" ]] && log_info "  GARDEN_RELEASE_TARBALL=$GARDEN_RELEASE_TARBALL"
    
    cd "${SCRIPT_DIR}/integration/garden"
    go run github.com/onsi/ginkgo/v2/ginkgo --trace -v .
}

# Get the VM IP address
get_vm_ip() {
    local ip
    ip=$(bosh -d "$DEPLOYMENT_NAME" instances --json 2>/dev/null | \
         jq -r ".Tables[0].Rows[] | select(.instance | startswith(\"${INSTANCE_GROUP}/\")) | .ips" | head -1)
    
    if [[ -z "$ip" || "$ip" == "null" ]]; then
        log_error "Could not find VM IP for ${INSTANCE_GROUP} in deployment ${DEPLOYMENT_NAME}"
        log_info "Available deployments:"
        bosh deployments --json 2>/dev/null | jq -r '.Tables[0].Rows[].name'
        exit 1
    fi
    
    echo "$ip"
}

# Cross-compile the agent
build_agent() {
    log_info "Building bosh-agent for linux/amd64..."
    cd "$SCRIPT_DIR"
    
    # Use the existing build script
    GOARCH=amd64 GOOS=linux bin/build
    
    if [[ ! -f "out/bosh-agent" ]]; then
        log_error "Build failed - out/bosh-agent not found"
        exit 1
    fi
    
    log_info "Build successful: out/bosh-agent ($(ls -lh out/bosh-agent | awk '{print $5}'))"
}

# Deploy agent to VM
deploy_agent() {
    local vm_ip="$1"
    
    log_info "Deploying agent to ${INSTANCE_GROUP}/${INSTANCE_ID} at ${vm_ip}..."
    
    # Use bosh ssh to copy and install the agent
    # This works even when the agent is broken because os-conf user_add creates SSH users
    
    log_info "Stopping bosh-agent service..."
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "sudo systemctl stop bosh-agent || sudo sv stop agent || true" 2>/dev/null || true
    
    log_info "Copying new agent binary..."
    # Create a temp file with the agent
    local temp_agent="/tmp/bosh-agent-$$"
    cp out/bosh-agent "$temp_agent"
    
    # Use bosh scp to copy the file
    bosh -d "$DEPLOYMENT_NAME" scp "$temp_agent" "${INSTANCE_GROUP}/${INSTANCE_ID}:/tmp/bosh-agent-new"
    rm -f "$temp_agent"
    
    log_info "Installing new agent..."
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "
        sudo mv /tmp/bosh-agent-new /var/vcap/bosh/bin/bosh-agent
        sudo chmod +x /var/vcap/bosh/bin/bosh-agent
        sudo chown root:root /var/vcap/bosh/bin/bosh-agent
    "
    
    log_info "Agent installed successfully!"
}

# Configure agent.json for NATS firewall
configure_agent() {
    local vm_ip="$1"
    
    log_info "Configuring agent.json to enable NATS firewall..."
    
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c '
        sudo python3 << "PYEOF"
import json

config_path = "/var/vcap/bosh/agent.json"

with open(config_path, "r") as f:
    config = json.load(f)

if "Platform" not in config:
    config["Platform"] = {}
if "Linux" not in config["Platform"]:
    config["Platform"]["Linux"] = {}

config["Platform"]["Linux"]["EnableNATSFirewall"] = True

with open(config_path, "w") as f:
    json.dump(config, f, indent=2)

print("EnableNATSFirewall set to true")
PYEOF
    '
}

# Start the agent and show logs
start_agent() {
    local vm_ip="$1"
    
    log_info "Starting bosh-agent service..."
    
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "
        sudo systemctl start bosh-agent || sudo sv start agent
        sleep 2
        sudo systemctl status bosh-agent || sudo sv status agent || true
    "
    
    log_info "Recent agent logs:"
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "
        sudo journalctl -u bosh-agent --no-pager -n 30 2>/dev/null || sudo tail -30 /var/vcap/bosh/log/current
    " 2>/dev/null | tail -40
}

# Check nftables status
check_nftables() {
    local vm_ip="$1"
    
    log_info "Checking nftables status..."
    
    bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "
        echo '=== nft list tables ==='
        sudo nft list tables
        echo ''
        echo '=== nft list table inet bosh_agent (if exists) ==='
        sudo nft list table inet bosh_agent 2>&1 || echo 'Table does not exist'
    " 2>/dev/null | tail -30
}

# Main
main() {
    local action="${1:-deploy}"
    
    check_bosh
    
    # Commands that don't need an existing deployment
    case "$action" in
        create)
            create_deployment
            return
            ;;
        delete)
            delete_deployment
            return
            ;;
        help|-h|--help)
            show_help
            return
            ;;
    esac
    
    local vm_ip
    vm_ip=$(get_vm_ip)
    log_info "Target VM IP: $vm_ip"
    
    case "$action" in
        build)
            build_agent
            ;;
        deploy)
            build_agent
            deploy_agent "$vm_ip"
            configure_agent "$vm_ip"
            start_agent "$vm_ip"
            check_nftables "$vm_ip"
            ;;
        start)
            start_agent "$vm_ip"
            ;;
        stop)
            log_info "Stopping agent..."
            bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "sudo systemctl stop bosh-agent || sudo sv stop agent || true"
            ;;
        logs)
            log_info "Fetching agent logs..."
            bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}" -c "
                sudo journalctl -u bosh-agent --no-pager -n 100 2>/dev/null || sudo tail -100 /var/vcap/bosh/log/current
            "
            ;;
        nft|nftables)
            check_nftables "$vm_ip"
            ;;
        garden)
            check_garden "$vm_ip"
            ;;
        test-garden)
            run_garden_tests
            ;;
        ssh)
            log_info "Opening SSH session..."
            bosh -d "$DEPLOYMENT_NAME" ssh "${INSTANCE_GROUP}/${INSTANCE_ID}"
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
}

main "$@"
