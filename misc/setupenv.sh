#!/bin/bash

USER=work
WORK_ROOT=/letv

THIS_DIR=$(dirname $(readlink -f $0) )

mkdir -p $WORK_ROOT/log/gibbon
chown -R $USER:$USER $WORK_ROOT/log/gibbon

mkdir -p $WORK_ROOT/run/gibbon
chown -R $USER:$USER $WORK_ROOT/run/gibbon

mkdir -p $WORK_ROOT/gibbon
cp $THIS_DIR/gibbond $WORK_ROOT/gibbon
cp $THIS_DIR/control.sh $WORK_ROOT/gibbon
cp -r $THIS_DIR/etc $WORK_ROOT/gibbon
chown -R $USER:$USER $WORK_ROOT/gibbon
