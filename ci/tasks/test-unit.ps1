$ErrorActionPreference='Stop'
trap {
    write-error $_
    exit 1
}

cd bosh-agent

go.exe run github.com/onsi/ginkgo/v2/ginkgo -r -race --keep-going --skip-package="integration,vendor"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Ginkgo returned non-zero exit code: $LASTEXITCODE"
    Write-Error $_
    exit 1
}
