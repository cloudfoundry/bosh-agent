$ErrorActionPreference='Stop'
trap {
    write-error $_
    exit 1
}

cd bosh-agent

go.exe run github.com/onsi/ginkgo/ginkgo -r -race -keepGoing -skipPackage="integration,vendor"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Gingko returned non-zero exit code: $LASTEXITCODE"
    Write-Error $_
    exit 1
}
