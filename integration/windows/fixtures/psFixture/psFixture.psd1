@{
    RootModule = 'psFixture'
    ModuleVersion = '0.1'
    GUID = '1113e65d-b18e-4217-abc8-12c60a8f1f52'
    Author = 'BOSH'
    Copyright = '(c) 2017 BOSH'
    Description = 'Fixtures for bosh agent on windows'
    PowerShellVersion = '4.0'
    FunctionsToExport = @(,'Protect-Path', 'Check-Acls')
    CmdletsToExport = @()
    VariablesToExport = '*'
    AliasesToExport = @()
}