param(
    [switch]$PrintOnly
)

$ErrorActionPreference = "Stop"

function Add-PathEntry {
    param(
        [string]$Entry
    )

    if (-not $Entry) {
        return
    }

    if (-not (Test-Path $Entry)) {
        return
    }

    $parts = @($env:Path -split ';' | Where-Object { $_ })
    if ($parts -contains $Entry) {
        return
    }

    $env:Path = "$Entry;$env:Path"
}

function Test-SimpleAbsolutePath {
    param(
        [string]$Value
    )

    if (-not $Value) {
        return $false
    }

    if ($Value.Contains(';')) {
        return $false
    }

    return $Value -match '^[A-Za-z]:\\'
}

$goRoot = $null
$goCandidates = @(
    "C:\Program Files\Go",
    "C:\Go"
)

foreach ($candidate in $goCandidates) {
    if (Test-Path (Join-Path $candidate "bin\go.exe")) {
        $goRoot = $candidate
        break
    }
}

if (-not $goRoot) {
    throw "Go was not found. Install Go first, for example with: choco install golang -y"
}

$mingwRoot = $null
$mingwCandidates = @(
    "C:\ProgramData\mingw64\mingw64",
    "C:\msys64\ucrt64",
    "C:\msys64\mingw64"
)

foreach ($candidate in $mingwCandidates) {
    if (Test-Path (Join-Path $candidate "bin\gcc.exe")) {
        $mingwRoot = $candidate
        break
    }
}

$vsCl = (Get-Command cl.exe -ErrorAction SilentlyContinue | Select-Object -First 1)

Add-PathEntry (Join-Path $goRoot "bin")

if ($mingwRoot) {
    Add-PathEntry (Join-Path $mingwRoot "bin")
    $env:CC = "gcc"
    $env:CXX = "g++"
} elseif ($vsCl) {
    $env:CC = $vsCl.Source
    $env:CXX = $vsCl.Source
}

$env:GOROOT = $goRoot
if (-not (Test-SimpleAbsolutePath $env:GOPATH)) {
    $env:GOPATH = Join-Path $env:USERPROFILE "go"
}
$env:CGO_ENABLED = "1"

if ($PrintOnly) {
    $summary = [ordered]@{
        GOROOT      = $env:GOROOT
        GOPATH      = $env:GOPATH
        CGO_ENABLED = $env:CGO_ENABLED
        CC          = $env:CC
        CXX         = $env:CXX
        PATH_HEAD   = (($env:Path -split ';' | Select-Object -First 6) -join ';')
    }
    $summary.GetEnumerator() | ForEach-Object {
        "{0}={1}" -f $_.Key, $_.Value
    }
    return
}

Write-Host "re_gent dev environment loaded" -ForegroundColor Green
Write-Host "  GOROOT=$env:GOROOT"
Write-Host "  GOPATH=$env:GOPATH"
Write-Host "  CGO_ENABLED=$env:CGO_ENABLED"
if ($env:CC) {
    Write-Host "  CC=$env:CC"
}
if ($env:CXX) {
    Write-Host "  CXX=$env:CXX"
}
if (-not $mingwRoot -and $vsCl) {
    Write-Host "  note: using cl.exe fallback; if CGO linking fails, start from a VS Developer PowerShell or install MinGW" -ForegroundColor Yellow
}
if (-not $mingwRoot -and -not $vsCl) {
    Write-Host "  warning: no C compiler detected; pure Go builds should still work, but CGO builds may fail" -ForegroundColor Yellow
}
Write-Host ""
Write-Host "Next commands:"
Write-Host "  go version"
Write-Host "  gcc --version"
Write-Host "  go test ./..."
Write-Host "  go build -o .\bin\rgt.exe .\cmd\rgt"
