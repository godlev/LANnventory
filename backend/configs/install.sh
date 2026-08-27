#!/bin/sh

cp lannventory /usr/bin/
ln -sf /usr/bin/lannventory /usr/bin/watchyourlan
cp lannventory.service /lib/systemd/system/
