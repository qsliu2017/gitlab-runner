param(
    [string]$testsDefinitionsFile = ".\testsdefinitions-" + $env:CI_NODE_INDEX
)

$InformationPreference = "Continue"

function Get-Line([string]$file) {
    (Get-Content $file | Measure-Object -Line).Lines
}

$numberOfDefinitions = Get-Line -file $testsDefinitionsFile

Write-Information "Number of definitions: $numberOfDefinitions"
Write-Information "Suite size: $env:CI_NODE_TOTAL"
Write-Information "Suite index: $env:CI_NODE_INDEX"

Write-Information "Execution size: $executionSize"
Write-Information "Execution offset: $executionOffset"

Set-MpPreference -DisableRealtimeMonitoring $true

winrm get winrm/config/winrs

$type="integration"
if ($env:TESTFLAGS.Contains('!integration')) {
    $type="unit"
}

New-Item -ItemType "directory" -Path ".\" -Name ".testoutput\${type}"

$failed = @()
Get-Content $testsDefinitionsFile | ForEach-Object {
    $tests, $pkg = $_.Split(" ", 2)
    $index = $env:CI_NODE_INDEX

    Write-Information "`r`n`r`n--- Starting part $index of go $type tests of '$pkg' package:`r`n`r`n"

    go test $env:TESTFLAGS -timeout 30m -v -run "$tests" $pkg.split(" ") > ".testoutput/${type}/${index}.windows.${WINDOWS_VERSION}.output.txt"

    if ($LASTEXITCODE -ne 0) {
        $failed += "$index $pkg"
    }
}

if ($failed.count -ne 0) {
    Write-Output ""
    Write-Warning "Failed split test of packages:"
    $failed | Out-String | Write-Warning

    exit 1
}
