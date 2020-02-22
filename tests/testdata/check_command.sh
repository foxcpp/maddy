#!/bin/sh

if [ -e "${TEST_PWD}/testdata/${1}.hdr" ]; then
    cat "${TEST_PWD}/testdata/${1}.hdr"
fi

cat > ${TEST_STATE_DIR}/msg

if [ -e "${TEST_PWD}/testdata/${1}.exit" ]; then
    exit "$(cat "${TEST_PWD}/testdata/${1}.exit")"
fi
