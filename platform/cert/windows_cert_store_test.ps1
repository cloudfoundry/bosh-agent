$ErrorActionPreference = 'Stop'
$tokens = $null
$errors = $null
$scriptPath = Join-Path $PSScriptRoot 'windows_cert_store.ps1'
$null = [Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
. $scriptPath

function Expect-Failure($Block, $Message) {
    try { & $Block; throw 'Expected rejection did not occur' }
    catch { if ($_.Exception.Message -notlike "*$Message*") { throw } }
}
function New-TestRule($Sid, $Rights = 'FullControl') {
    $rightsValue = if ($Rights -is [int]) { $Rights } else { [Security.AccessControl.FileSystemRights]$Rights }
    [PSCustomObject]@{ IdentityReference = [PSCustomObject]@{ Value = $Sid }; FileSystemRights = $rightsValue; AccessControlType = 'Allow' }
}
function Reset-TestAcl {
    $script:testItem = [PSCustomObject]@{ Attributes = [IO.FileAttributes]::Directory; PSIsContainer = $true; Length = 1 }
    $script:testOwner = 'S-1-5-18'
    $script:testRules = @((New-TestRule 'S-1-5-18'), (New-TestRule 'S-1-5-32-544'))
    $script:testAcl = [PSCustomObject]@{ AreAccessRulesProtected = $true }
    $script:testAcl | Add-Member ScriptMethod GetOwner { param($Type) [PSCustomObject]@{ Value = $script:testOwner } }
    $script:testAcl | Add-Member ScriptMethod GetAccessRules { param($Explicit, $Inherited, $Type) $script:testRules }
}
function Get-Item { param($LiteralPath, [switch]$Force, $ErrorAction) $script:testItem }
function Get-Acl { param($LiteralPath, $ErrorAction) $script:testAcl }

Reset-TestAcl
$null = Assert-CertificatePathAcl 'fixture' $true
$script:testAcl.AreAccessRulesProtected = $false
Expect-Failure { Assert-CertificatePathAcl 'fixture' $true } 'protected DACL'
Reset-TestAcl
$script:testOwner = 'S-1-5-21-1234'
Expect-Failure { Assert-CertificatePathAcl 'fixture' $true } 'untrusted owner'
Reset-TestAcl
$script:testRules += New-TestRule 'S-1-5-11' 'ReadAndExecute'
Expect-Failure { Assert-CertificatePathAcl 'fixture' $true } 'only SYSTEM'
# A read-only ordinary parent grant is compatible with protected children.
$null = Assert-CertificatePathAcl 'fixture' $false
$script:testRules += New-TestRule 'S-1-5-11' 'Write'
Expect-Failure { Assert-CertificatePathAcl 'fixture' $false } 'untrusted modification'
Reset-TestAcl
$script:testRules += New-TestRule 'S-1-5-11' 'DeleteSubdirectoriesAndFiles'
Expect-Failure { Assert-CertificatePathAcl 'fixture' $false $true } 'untrusted modification'
Reset-TestAcl
$script:testRules = @()
Expect-Failure { Assert-CertificatePathAcl 'fixture' $false } 'nonempty DACL'
Reset-TestAcl
$script:testRules += New-TestRule 'S-1-5-11' 0x10000000
Expect-Failure { Assert-CertificatePathAcl 'fixture' $false $true } 'untrusted modification'
Reset-TestAcl
$script:testItem.Attributes = [IO.FileAttributes]::ReparsePoint
Expect-Failure { Assert-CertificatePathAcl 'fixture' $true } 'reparse points'
Reset-TestAcl
$script:testRules = @((New-TestRule 'S-1-5-18'))
Expect-Failure { Assert-CertificatePathAcl 'fixture' $true } 'full control'

# These paths stop before opening any root store. No live store is read/written.
function Test-Path { param($LiteralPath) $LiteralPath -eq 'legacy' }
Expect-Failure { Invoke-RootCertificateUpdate ([PSCustomObject]@{ BackupDirectory = 'private'; LegacyPath = 'legacy'; Certificates = @() }) } 'Legacy root certificate backup is untrusted'
$script:directoryValidated = $false
function Initialize-CertificateBackupDirectory { param($BackupDirectory) $script:directoryValidated = $true }
function Test-Path { param($LiteralPath) $false }
Expect-Failure { Invoke-RootCertificateUpdate ([PSCustomObject]@{ BackupDirectory = 'private'; LegacyPath = 'absent'; Certificates = @() }) } 'current root store will not be recaptured'
if (-not $script:directoryValidated) { throw 'Directory validation was skipped' }

# Explicit provisioning rejects a wrong approved digest before installation or
# certificate-store use, using a real small input rather than an exhaustion run.
$fixture = [IO.Path]::GetTempFileName()
try {
    [IO.File]::WriteAllBytes($fixture, [byte[]](1,2,3))
    Expect-Failure { Install-RootCertificateBaseline 'private' $fixture ('0' * 64) } 'does not match'
} finally { [IO.File]::Delete($fixture) }
Write-Output 'Certificate backup safety checks passed without accessing a certificate store.'
