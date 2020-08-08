#!/bin/bash

DESCR=$(git describe --long 2>/dev/null)
if [ $? -ne 0 ]; then
	echo "source-build"
	exit
fi
set -e

MADDY_MAJOR=$(sed 's/^v//' <<<$DESCR | cut -f1 -d '.')
MADDY_MINOR=$(cut -f2 -d '.' <<<$DESCR )

MADDY_PATCH=$(cut -f1 -d '-' <<<$DESCR | sed 's/-.+//' | cut -f3 -d '.')
MADDY_SNAPSHOT=$(cut -f2 -d '-' <<<$DESCR)
MADDY_COMMIT=$(cut -f3 -d '-' <<<$DESCR)

if [ $MADDY_SNAPSHOT -ne 0 ]; then
    (( MADDY_MINOR++ ))
    MADDY_PATCH=0

    MADDY_VER="$MADDY_MAJOR.$MADDY_MINOR.$MADDY_PATCH-dev$MADDY_SNAPSHOT+$MADDY_COMMIT"
else
    MADDY_VER="$MADDY_MAJOR.$MADDY_MINOR.$MADDY_PATCH+$MADDY_COMMIT"
fi

echo $MADDY_VER
