#!/bin/bash

set -e

rm -rf bin
mkdir -p bin/release

VERSION=$1
VERSION_HASH="$(git rev-parse --short HEAD)"
VERSION_DATE="$(date -u '+%d.%m.%Y %H:%M:%S')"

echo -e "\nVERSION=$VERSION"
echo "VERSION_HASH=$VERSION_HASH"
echo "VERSION_DATE=$VERSION_DATE"

if [ -z $VERSION ]; then
    echo "Error: First argument must be release version"
    exit 1
fi

for goos in linux darwin windows freebsd openbsd netbsd ; do
    for goarch in amd64 386; do
        # path
        outdir="bin/$goos/$goarch"
        path="$outdir/prometheus_varnish_exporter"
        if [ $goos = windows ] ; then
            path=$path.exe
        fi

        mkdir -p $outdir
        cp LICENSE CHANGELOG.md README.md $outdir/

        # build
        echo -e "\nBuilding $goos/$goarch"
        GOOS=$goos GOARCH=$goarch go build -o $path -ldflags "-X 'main.Version=$VERSION' -X 'main.VersionHash=$VERSION_HASH' -X 'main.VersionDate=$VERSION_DATE'"
        echo "  > `du -hc $path | awk 'NR==1{print $1;}'`    `file $path`"

        # compress (for unique filenames to github release files)
        if [ $goos = windows ] ; then
            zip -rjX ./bin/release/$goos-$goarch.zip ./$outdir/ > /dev/null 2>&1
        else
            tar -C ./$outdir/ -cvzf ./bin/release/$goos-$goarch.tar.gz . > /dev/null 2>&1
        fi
    done
done

go env > .goenv
source .goenv
rm .goenv

echo -e "\nRelease done: $(./bin/$GOOS/$GOARCH/prometheus_varnish_exporter --version)"
for goos in linux darwin windows freebsd openbsd netbsd ; do
    for goarch in amd64 386; do
        path=bin/release/$goos-$goarch.tar.gz
        if [ $goos = windows ] ; then
            path=bin/release/$goos-$goarch.zip
        fi
        echo "  > `du -hc $path | awk 'NR==1{print $1;}'`    $path"
    done
done
