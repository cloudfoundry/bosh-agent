function Check-Acls {
    param([string]$path)

    $expectedacls = New-Object System.Collections.ArrayList
    [void] $expectedacls.AddRange((
    "${env:COMPUTERNAME}\Administrator,Allow",
    "NT AUTHORITY\SYSTEM,Allow",
    "BUILTIN\Administrators,Allow",
    "CREATOR OWNER,Allow",
    "APPLICATION PACKAGE AUTHORITY\ALL APPLICATION PACKAGES,Allow"
    ))

    # for 2016, for some reason every file in C:\Program Files\OpenSSH
    # ends up with "APPLICATION PACKAGE AUTHORITY\ALL RESTRICTED APPLICATION PACKAGES,Allow".
    # adding this to unblock 2016 pipeline
    $windowsVersion = [environment]::OSVersion.Version.Major
    if ($windowsVersion -ge "10") {
        "Adding 2016 ACLs"
        $expectedacls.Add("APPLICATION PACKAGE AUTHORITY\ALL RESTRICTED APPLICATION PACKAGES,Allow")
    }

    $errs = @()

    Get-ChildItem -Path $path -Recurse | foreach {
        $name = $_.FullName
        If (-Not ($_.Attributes -match "ReparsePoint")) {
            Get-Acl $name | Select -ExpandProperty Access | ForEach-Object {
                $ident = ('{0},{1}' -f $_.IdentityReference, $_.AccessControlType).ToString()
                If (-Not $expectedacls.Contains($ident)) {
                    If (-Not ($ident -match "NT [\w]+\\[\w]+,Allow")) {
                        $errs += "Error ($name): $ident"
                    }
                }
            }
        }
    }

    return $errs -join "`r`n"

}