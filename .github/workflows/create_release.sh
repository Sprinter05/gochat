#!/bin/sh

touch RELEASE.txt
for v in $(find artifacts/)
do
	if [ -d $v ]; then continue; fi
	BASENAME=$(echo $(dirname $v) | cut -d "/" -f 2)
	echo $BASENAME | tr "\n" " " >> RELEASE.txt
	sha256sum $v 2> /dev/null | cut -d " " -f 1 >> RELEASE.txt
	tar --transform 's/.*\///g' -zcf "${BASENAME}.tar.gz" $v
done
