#!/bin/sh
set -e
recursive_for_loop() {
    ls -1| while read f; do
        if [ -d $f  -a ! -h $f ]; then
            #echo $f
            cd -- "$f"
            go test
            recursive_for_loop
            cd ..
        fi
    done
}
recursive_for_loop
