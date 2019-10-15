#!/bin/bash
set -ueo pipefail
set +x

if [ $# -ne 2 ]
then
      echo "Usage: release.sh version version-code"
      exit 1
fi

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

VERSION=$1
VERSION_CODE=$2

make libuplink-gomobile

# create versioned copies of files
cp -v "$SCRIPTDIR/libuplink-android-gomobile.pom"   "$TMP/libuplink-android-gomobile-$VERSION.pom"
cp -v "libuplink-android.aar"                       "$TMP/libuplink-android-gomobile-$VERSION.aar"
cp -v "libuplink-android-sources.jar"               "$TMP/libuplink-android-gomobile-$VERSION-sources.jar"
cp -v "$SCRIPTDIR/AndroidManifest.xml"              "$TMP/AndroidManifest.xml"

# set version for pom file
sed -i 's@<version></version>@<version>'$VERSION'</version>@g' "$TMP/libuplink-android-gomobile-$VERSION.pom"
# set version for AndroidManifest.xml
sed -i 's@android:versionName=""@android:versionName="'$VERSION'"@g' "$TMP/AndroidManifest.xml"
# set versionCode for AndroidManifest.xml
sed -i 's@android:versionCode=""@android:versionCode="'$VERSION_CODE'"@g' "$TMP/AndroidManifest.xml"

cd "$TMP"
zip -ur libuplink-android-gomobile-$VERSION.aar AndroidManifest.xml

TARGET_URL="https://api.bintray.com/content/storj/maven/libuplink-android-gomobile/$VERSION/io/storj/libuplink-android-gomobile/$VERSION"

curl -T "$TMP/libuplink-android-gomobile-$VERSION.pom"          -u$BINTRAY_USER:$BINTRAY_API_KEY "$TARGET_URL/libuplink-android-gomobile-$VERSION.pom"
curl -T "$TMP/libuplink-android-gomobile-$VERSION.aar"          -u$BINTRAY_USER:$BINTRAY_API_KEY "$TARGET_URL/libuplink-android-gomobile-$VERSION.aar"
curl -T "$TMP/libuplink-android-gomobile-$VERSION-sources.jar"  -u$BINTRAY_USER:$BINTRAY_API_KEY "$TARGET_URL/libuplink-android-gomobile-$VERSION-sources.jar"
# TODO enable this after verifying that upload was done correctly
# curl -X POST -u$BINTRAY_USER:$BINTRAY_API_KEY "https://api.bintray.com/content/storj/maven/libuplink-android-gomobile/$VERSION/publish"