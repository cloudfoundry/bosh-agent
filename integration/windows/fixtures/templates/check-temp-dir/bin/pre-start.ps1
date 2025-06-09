Write-Host "TMP = $Env:TMP"
Write-Host "TEMP = $Env:TEMP"
Write-Host "SystemTempSymlink = $(Get-Item \Windows\SystemTemp | Select-Object -ExpandProperty Target)"
