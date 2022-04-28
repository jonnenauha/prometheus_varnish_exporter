#!/bin/bash

set -e

if [ ! -e main.go ] ; then
    echo "Error: Script can only be ran on the root of the source tree"
    exit 1
fi

rm -rf bin
mkdir -p bin/build bin/release

VERSION=$1
VERSION_HASH="$(git rev-parse --short HEAD)"
VERSION_DATE="$(date -u '+%d.%m.%Y %H:%M:%S')"

echo -e "\nVERSION=$VERSION"
echo "VERSION_HASH=$VERSION_HASH"
echo "VERSION_DATE=$VERSION_DATE"

if [ -z "$VERSION" ]; then
    echo "Error: First argument must be release version"
    exit 1
fi

tar -cvzf ./bin/release/dashboards-$VERSION.tar.gz dashboards/* > /dev/null 2>&1

for goos in linux darwin windows freebsd openbsd netbsd ; do
    for goarch in amd64 arm64; do
        # path
        file_versioned="prometheus_varnish_exporter-$VERSION.$goos-$goarch"
        outdir="bin/build/$file_versioned"
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
        tar -C ./bin/build -cvzf ./bin/release/$file_versioned.tar.gz $file_versioned > /dev/null 2>&1
    done
done

go env > .goenv
source .goenv
rm .goenv

echo -e "\nRelease done: $(./bin/build/prometheus_varnish_exporter-$VERSION.$GOOS-$GOARCH/prometheus_varnish_exporter --version)"
for goos in linux darwin windows freebsd openbsd netbsd ; do
    for goarch in amd64 arm64; do
        file_versioned="prometheus_varnish_exporter-$VERSION.$goos-$goarch"
        path=bin/release/$file_versioned.tar.gz
        echo "  > `du -hc $path | awk 'NR==1{print $1;}'`    $path"
    done
done

cd ./bin/release
shasum --algorithm 256 --binary ./* | sed -En "s/\*\.\/(.*)$/\1/p" > sha256sums.txt
cd ../..
