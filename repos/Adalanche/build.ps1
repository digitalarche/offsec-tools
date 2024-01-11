function BuildVariants {
  param (
    $ldflags,
    $compileflags,
    $prefix,
    $suffix,
    $arch,
    $os,
    $path
  )

  foreach ($currentarch in $arch) {
    foreach ($currentos in $os) {
      $env:GOARCH = $currentarch
      $env:GOOS = $currentos
      go build -ldflags "$ldflags" -o binaries/$prefix-$currentos-$currentarch-$VERSION$suffix $compileflags $path
      if (Get-Command "cyclonedx-gomod" -ErrorAction SilentlyContinue)
      {
        cyclonedx-gomod app -json -licenses -output binaries/$prefix-$currentos-$currentarch-$VERSION$suffix.bom.json -main $path .
      }
    }
  }
}

Set-Location $PSScriptRoot

$COMMIT = git rev-parse --short HEAD
$VERSION = git describe --tags --exclude latest
$DIRTYFILES = git status --porcelain

if ("$DIRTYFILES" -ne "") {
  $VERSION = "$VERSION-local-changes"
}

$LDFLAGS = "-X github.com/lkarlslund/adalanche/modules/version.Commit=$COMMIT -X github.com/lkarlslund/adalanche/modules/version.Version=$VERSION"

# Release
BuildVariants -ldflags "$LDFLAGS -s" -prefix adalanche-collector -path ./collector -arch @("386") -os @("windows") -suffix ".exe"
BuildVariants -ldflags "$LDFLAGS -s" -prefix adalanche -path ./adalanche -arch @("amd64", "arm64") -os @("windows") -suffix ".exe"
BuildVariants -ldflags "$LDFLAGS -s" -prefix adalanche -path ./adalanche -arch @("amd64", "arm64") -os @("darwin", "linux")
