$ErrorActionPreference='Stop'
trap {
    write-error $_
    exit 1
}

# Install chocolatey package manager
#
# TODO:
# Ask Concourse team if we don't have to do this and have a better way to
# install git.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Set-ExecutionPolicy Bypass -Scope Process -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))
choco install -y git

$env:PATH = $env:PATH + ";c:\program files\git\bin;"
refreshenv

cd bosh-agent

go.exe install github.com/onsi/ginkgo/ginkgo
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
