#!/bin/bash

USER=work
WORK_ROOT=/letv

mkdir -p $WORK_ROOT/log/gibbon
chown -R $USER:$USER $WORK_ROOT/log/gibbon

mkdir -p $WORK_ROOT/run/gibbon
chown -R $USER:$USER $WORK_ROOT/run/gibbon

mkdir -p $WORK_ROOT/gibbon
cp gibbon $WORK_ROOT/gibbon
cp control.sh $WORK_ROOT/gibbon
cp -r etc $WORK_ROOT/gibbon
chown -R $USER:$USER $WORK_ROOT/gibbon
