#!/bin/bash
# Script to compile a garden-runc release using Docker and bosh-agent compile
# Based on https://bosh.io/docs/compiled-releases/#bosh-agent-compile
#
# Usage:
#   ./bin/compile-garden-release.sh [RELEASE_DIR] [OUTPUT_DIR]
#
# Arguments:
#   RELEASE_DIR - Path to garden-runc-release source (default: ~/workspace/garden-runc-release)
#   OUTPUT_DIR  - Output directory for compiled tarball (default: ./compiled-releases)
#
# Environment variables:
#   STEMCELL_OS      - Stemcell OS to compile for (default: ubuntu-noble)
#   STEMCELL_VERSION - Stemcell version (default: latest)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

# Arguments
RELEASE_DIR="${1:-${HOME}/workspace/garden-runc-release}"
OUTPUT_DIR="${2:-${REPO_DIR}/compiled-releases}"

# Stemcell configuration
STEMCELL_OS="${STEMCELL_OS:-ubuntu-noble}"
STEMCELL_VERSION="${STEMCELL_VERSION:-latest}"

# GitHub Container Registry image for stemcells
# See: https://github.com/orgs/cloudfoundry/packages?repo_name=bosh-linux-stemcell-builder
STEMCELL_IMAGE="ghcr.io/cloudfoundry/${STEMCELL_OS}-stemcell:${STEMCELL_VERSION}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $*" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*" >&2; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Check prerequisites
check_prerequisites() {
    if ! command -v docker &> /dev/null; then
        log_error "docker is required but not found"
        exit 1
    fi
    
    if ! command -v bosh &> /dev/null; then
        log_error "bosh CLI is required but not found"
        exit 1
    fi
    
    if [[ ! -d "$RELEASE_DIR" ]]; then
        log_error "Release directory not found: $RELEASE_DIR"
        log_info "Clone it with: git clone --recurse-submodules https://github.com/rkoster/garden-runc-release -b noble-nested-warden"
        exit 1
    fi
}

# Create source release tarball
create_source_release() {
    local release_tarball
    
    log_info "Creating source release tarball from $RELEASE_DIR..."
    cd "$RELEASE_DIR"
    
    # Get release name and version info
    local release_name
    release_name=$(grep '^name:' config/final.yml 2>/dev/null | awk '{print $2}' || echo "garden-runc")
    
    local commit_hash
    commit_hash=$(git rev-parse --short HEAD)
    
    # Create dev release with a predictable version
    local version="0+dev.${commit_hash}"
    release_tarball="${RELEASE_DIR}/dev_releases/${release_name}/${release_name}-${version}.tgz"
    
    # Check if we already have a recent dev release
    if [[ -f "$release_tarball" ]]; then
        log_info "Using existing dev release: $release_tarball"
    else
        log_info "Creating dev release (version: ${version})..."
        bosh create-release --force --tarball="${release_tarball}" --version="${version}"
    fi
    
    echo "$release_tarball"
}

# Compile release using Docker and bosh-agent compile
compile_release() {
    local source_tarball="$1"
    local source_filename
    source_filename=$(basename "$source_tarball")
    
    log_info "Compiling release using Docker..."
    log_info "  Stemcell image: $STEMCELL_IMAGE"
    log_info "  Source tarball: $source_tarball"
    log_info "  Output dir: $OUTPUT_DIR"
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Create a temporary directory for the compilation
    local work_dir
    work_dir=$(mktemp -d)
    trap "rm -rf '$work_dir'" EXIT
    
    # Copy source tarball to work directory
    cp "$source_tarball" "${work_dir}/${source_filename}"
    
    # Pull the stemcell image (if not already cached)
    log_info "Pulling stemcell image (if needed)..."
    if ! docker pull "$STEMCELL_IMAGE"; then
        log_error "Failed to pull stemcell image: $STEMCELL_IMAGE"
        log_info "Available Noble stemcell images:"
        log_info "  ghcr.io/cloudfoundry/ubuntu-noble-stemcell:latest"
        log_info "  ghcr.io/cloudfoundry/ubuntu-jammy-stemcell:latest"
        exit 1
    fi
    
    # Run bosh-agent compile in Docker
    # The bosh-agent binary is at /var/vcap/bosh/bin/bosh-agent in the stemcell image
    log_info "Running bosh-agent compile..."
    docker run --rm \
        --privileged \
        --security-opt seccomp=unconfined \
        --security-opt apparmor=unconfined \
        -v "${work_dir}:/releases" \
        "$STEMCELL_IMAGE" \
        /var/vcap/bosh/bin/bosh-agent compile \
            --output-directory=/releases \
            "/releases/${source_filename}"
    
    # Find the compiled release
    local compiled_tarball
    compiled_tarball=$(find "$work_dir" -name "*.tgz" ! -name "$source_filename" -type f | head -1)
    
    if [[ -z "$compiled_tarball" ]]; then
        log_error "No compiled release found in $work_dir"
        ls -la "$work_dir"
        exit 1
    fi
    
    local compiled_filename
    compiled_filename=$(basename "$compiled_tarball")
    
    # Move compiled release to output directory
    mv "$compiled_tarball" "${OUTPUT_DIR}/${compiled_filename}"
    
    log_info "Compiled release created: ${OUTPUT_DIR}/${compiled_filename}"
    echo "${OUTPUT_DIR}/${compiled_filename}"
}

# Verify the compiled release
verify_release() {
    local compiled_tarball="$1"
    
    log_info "Verifying compiled release..."
    
    # Check that it contains compiled_packages
    if tar -tzf "$compiled_tarball" 2>/dev/null | grep -q "compiled_packages/"; then
        log_info "  Release contains compiled packages"
    else
        log_error "  Release does not contain compiled packages!"
        exit 1
    fi
    
    # List the compiled packages
    log_info "Compiled packages:"
    tar -tzf "$compiled_tarball" 2>/dev/null | grep "compiled_packages/" | head -20 | while read -r pkg; do
        echo "    $pkg"
    done
}

# Main
main() {
    log_info "Garden-runc Release Compiler"
    log_info "============================"
    log_info ""
    
    check_prerequisites
    
    # Create source release
    local source_tarball
    source_tarball=$(create_source_release)
    
    # Compile
    local compiled_tarball
    compiled_tarball=$(compile_release "$source_tarball")
    
    # Verify
    verify_release "$compiled_tarball"
    
    log_info ""
    log_info "Success! Compiled release is at:"
    log_info "  $compiled_tarball"
    log_info ""
    log_info "Use this tarball with the gardeninstaller package:"
    log_info "  export GARDEN_RELEASE_TARBALL=$compiled_tarball"
    log_info "  go test ./integration/garden/..."
}

main "$@"
