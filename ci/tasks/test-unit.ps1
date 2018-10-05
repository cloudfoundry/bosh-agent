$ErrorActionPreference='Stop'
trap {
    write-error $_
    exit 1
}

powershell.exe bin/install-go.ps1
Set-ExecutionPolicy Bypass -Scope Process -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))
refreshenv

$env:GOPATH = Join-Path -Path $PWD "gopath"
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;C:/var/vcap/bosh/bin;" + $env:PATH

cd $env:GOPATH/src/github.com/cloudfoundry/bosh-agent

go.exe install github.com/cloudfoundry/bosh-agent/vendor/github.com/onsi/ginkgo/ginkgo
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error installing ginkgo"
    Write-Error $_
    exit 1
}

ginkgo.exe -r -race -keepGoing -skipPackage="integration,vendor"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Gingko returned non-zero exit code: $LASTEXITCODE"
    Write-Error $_
    exit 1
}
