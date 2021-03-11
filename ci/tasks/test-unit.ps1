$ErrorActionPreference='Stop'
trap {
    write-error $_
    exit 1
}

$env:GOPATH = Join-Path -Path $PWD "gopath"
$env:PATH = $env:GOPATH + "/bin;" + $env:PATH

cd $env:GOPATH/src/github.com/cloudfoundry/bosh-agent

go.exe run github.com/onsi/ginkgo/ginkgo -r -race -keepGoing -skipPackage="integration,vendor"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Gingko returned non-zero exit code: $LASTEXITCODE"
    Write-Error $_
    exit 1
}
