#!/bin/bash

outfile="data/validator_os_output.txt"

# declare -a targets=("ixmaps.ca" "cira.ca")
declare -a targets=("cira.ca" "ixmaps.ca" "heisse.de" "google.ca")

> $outfile

for i in "${targets[@]}"
do
    echo "$i" >> $outfile
    a=$(traceroute -I -q 1 -w 1 $i | awk '{print $3}')
    echo "$a" | tr -d "()" >> $outfile
    echo "--------------------" >> $outfile
    echo "" >> $outfile
done