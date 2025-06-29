#!/bin/sh

touch RELEASE.txt
echo "SHA-256 checksums of binaries:" >> RELEASE.txt
for v in $(find artifacts/)
do
	if [ -d $v ]; then continue; fi
	BASENAME=$(echo $(dirname $v) | cut -d "/" -f 2)
	SHANAME=$(sha256sum $v 2> /dev/null | cut -d " " -f 1)
	printf '**%s**: `%s`\n' $BASENAME $SHANAME >> RELEASE.txt
	tar --transform 's/.*\///g' -zcf "${BASENAME}.tar.gz" $v
done
