# Windows PowerShell 5 / .NET Framework: keep the baseline outside temporary data.
function Get-TrustedCertificatePrincipals {
    @('S-1-5-18', 'S-1-5-32-544') # Local SYSTEM and built-in Administrators
}

function Assert-CertificatePathAcl($Path, [bool]$Private, [bool]$ParentOnly = $false) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Certificate backup paths must not contain reparse points: $Path"
    }
    $acl = Get-Acl -LiteralPath $Path -ErrorAction Stop
    $trusted = @(Get-TrustedCertificatePrincipals)
    $owners = $trusted
    if (-not $Private) {
        # Windows owns volume/system ancestors through TrustedInstaller.
        $owners += 'S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464'
    }
    if ($owners -notcontains $acl.GetOwner([Security.Principal.SecurityIdentifier]).Value) {
        throw "Certificate backup path has an untrusted owner: $Path"
    }
    if ($Private -and -not $acl.AreAccessRulesProtected) {
        throw "Certificate backup must have a protected DACL: $Path"
    }
    $rights = [Security.AccessControl.FileSystemRights]
    $unsafe = $rights::Write -bor $rights::Delete -bor $rights::DeleteSubdirectoriesAndFiles -bor $rights::ChangePermissions -bor $rights::TakeOwnership
    if ($ParentOnly) {
        # Creating a different child does not permit replacing an existing,
        # protected child. Deletion/ownership/DACL changes do permit replacement.
        $unsafe = $rights::Delete -bor $rights::DeleteSubdirectoriesAndFiles -bor $rights::ChangePermissions -bor $rights::TakeOwnership
    }
    # Generic ALL/WRITE bits can also occur in filesystem ACLs.
    $unsafe = [int]$unsafe -bor 0x10000000 -bor 0x40000000
    $rules = @($acl.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier]))
    if ($rules.Count -eq 0) { throw "Certificate backup path must have a nonempty DACL: $Path" }
    $fullControl = @{}
    foreach ($rule in $rules) {
        $sid = $rule.IdentityReference.Value
        if ($Private -and ($trusted -notcontains $sid -or $rule.AccessControlType -ne 'Allow')) {
            throw "Certificate backup DACL must allow only SYSTEM and Administrators: $Path"
        }
        if ($rule.AccessControlType -eq 'Allow' -and $owners -notcontains $sid -and (([int]$rule.FileSystemRights -band [int]$unsafe) -ne 0)) {
            throw "Certificate backup path permits untrusted modification: $Path"
        }
        if ($rule.AccessControlType -eq 'Allow' -and (([int]$rule.FileSystemRights -band [int]$rights::FullControl) -eq [int]$rights::FullControl)) {
            $fullControl[$sid] = $true
        }
    }
    if ($Private -and (-not $fullControl['S-1-5-18'] -or -not $fullControl['S-1-5-32-544'])) {
        throw "Certificate backup DACL must grant SYSTEM and Administrators full control: $Path"
    }
    return $item
}

function New-CertificateBackupAcl([bool]$Directory) {
    if ($Directory) { $acl = New-Object Security.AccessControl.DirectorySecurity }
    else { $acl = New-Object Security.AccessControl.FileSecurity }
    $acl.SetOwner((New-Object Security.Principal.SecurityIdentifier('S-1-5-18')))
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($sid in Get-TrustedCertificatePrincipals) {
        if ($Directory) {
            $rule = New-Object Security.AccessControl.FileSystemAccessRule((New-Object Security.Principal.SecurityIdentifier($sid)), 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
        } else {
            $rule = New-Object Security.AccessControl.FileSystemAccessRule((New-Object Security.Principal.SecurityIdentifier($sid)), 'FullControl', 'Allow')
        }
        $acl.AddAccessRule($rule)
    }
    return $acl
}

function Initialize-CertificateBackupDirectory($BackupDirectory) {
    $parent = [IO.DirectoryInfo]::new([IO.Path]::GetDirectoryName($BackupDirectory))
    $first = $true
    for ($ancestor = $parent; $null -ne $ancestor; $ancestor = $ancestor.Parent) {
        $item = Assert-CertificatePathAcl $ancestor.FullName $false (-not $first)
        if (-not $item.PSIsContainer) { throw 'Certificate backup parent must be a directory' }
        $first = $false
    }
    if (-not (Test-Path -LiteralPath $BackupDirectory)) {
        # Never tighten and then trust an existing unprotected directory.
        # The validated parent prevents untrusted precreation during this call.
        [IO.DirectoryInfo]::new($BackupDirectory).Create((New-CertificateBackupAcl $true))
    }
    $directory = Assert-CertificatePathAcl $BackupDirectory $true
    if (-not $directory.PSIsContainer) { throw 'Certificate backup path must be a directory' }
}

# Explicit provisioning only: the caller supplies an independently approved
# baseline digest. The running agent never calls this function or captures the
# current store, even when the legacy temporary backup has disappeared.
function Install-RootCertificateBaseline($BackupDirectory, $BaselinePath, $ExpectedSha256) {
    $ErrorActionPreference = 'Stop'
    if ($ExpectedSha256 -notmatch '\A[0-9a-fA-F]{64}\z') { throw 'An independently verified SHA256 is required' }
    $baselineInput = [IO.File]::Open($BaselinePath, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    try {
        if ($baselineInput.Length -le 0 -or $baselineInput.Length -gt 32MB) { throw 'Baseline must be nonempty and at most 32 MiB' }
        $bytes = New-Object byte[] ([int]$baselineInput.Length)
        $offset = 0
        while ($offset -lt $bytes.Length) {
            $read = $baselineInput.Read($bytes, $offset, $bytes.Length - $offset)
            if ($read -eq 0) { throw 'Baseline ended unexpectedly' }
            $offset += $read
        }
    } finally { $baselineInput.Dispose() }
    $hasher = [Security.Cryptography.SHA256]::Create()
    try { $digest = [BitConverter]::ToString($hasher.ComputeHash($bytes)).Replace('-', '') }
    finally { $hasher.Dispose() }
    if ($digest -ne $ExpectedSha256) { throw 'Baseline SHA256 does not match independently verified digest' }
    $certificates = New-Object Security.Cryptography.X509Certificates.X509Certificate2Collection
    $certificates.Import($bytes)
    if ($certificates.Count -eq 0) { throw 'Baseline contains no certificates' }
    Initialize-CertificateBackupDirectory $BackupDirectory
    $backup = Join-Path $BackupDirectory 'rootCertBackup.sst'
    if (Test-Path -LiteralPath $backup) { throw 'A baseline already exists; refusing to overwrite it' }
    $pending = Join-Path $BackupDirectory (([Guid]::NewGuid().ToString()) + '.sst')
    try {
        [IO.File]::WriteAllBytes($pending, $bytes)
        Set-Acl -LiteralPath $pending -AclObject (New-CertificateBackupAcl $false) -ErrorAction Stop
        $null = Assert-CertificatePathAcl $pending $true
        Move-Item -LiteralPath $pending -Destination $backup -ErrorAction Stop
    } finally {
        if (Test-Path -LiteralPath $pending) { Remove-Item -LiteralPath $pending -Force -ErrorAction Stop }
    }
}

function Initialize-CertificateBackup($BackupDirectory, $LegacyPath) {
    if (Test-Path -LiteralPath $LegacyPath) {
        throw 'Legacy root certificate backup is untrusted. Restore an independently verified baseline into the protected root-cert-backup directory and remove/quarantine the legacy rootCertBackup.sst before restarting. See docs/windows-root-certificates.md. No trust-store changes were made.'
    }
    Initialize-CertificateBackupDirectory $BackupDirectory
    $backup = Join-Path $BackupDirectory 'rootCertBackup.sst'
    if (-not (Test-Path -LiteralPath $backup)) {
        throw 'Protected root certificate baseline is missing. Provision an independently verified baseline before starting the agent; the current root store will not be recaptured. See docs/windows-root-certificates.md.'
    }
    $file = Assert-CertificatePathAcl $backup $true
    if ($file.PSIsContainer -or $file.Length -le 0 -or $file.Length -gt 32MB) { throw 'Certificate baseline must be a nonempty file of at most 32 MiB' }
    return $backup
}

function Invoke-RootCertificateUpdate($Config) {
    $ErrorActionPreference = 'Stop'
    # Parse all incoming certificates before changing the root store. They travel
    # over stdin and remain in memory, without an untrusted temporary PEM file.
    $incoming = New-Object Security.Cryptography.X509Certificates.X509Certificate2Collection
    foreach ($encoded in $Config.Certificates) {
        $incoming.Add([Security.Cryptography.X509Certificates.X509Certificate2]::new([Convert]::FromBase64String($encoded))) | Out-Null
    }
    $backup = Initialize-CertificateBackup $Config.BackupDirectory $Config.LegacyPath
    $baseline = New-Object Security.Cryptography.X509Certificates.X509Certificate2Collection
    $baseline.Import($backup)
    if ($baseline.Count -eq 0) { throw 'Protected root certificate baseline is empty; no trust-store changes were made' }
    $store = [Security.Cryptography.X509Certificates.X509Store]::new('Root', 'LocalMachine')
    try {
        $store.Open([Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
        $store.RemoveRange($store.Certificates)
        $store.AddRange($baseline)
        $store.AddRange($incoming)
    } finally { $store.Close() }
}
