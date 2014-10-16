#!/bin/bash

PROGRAM=gibbon
cmd=$1

THIS_DIR=$(dirname $(readlink -f $0) )

function start()
{
	sleep 3
	supervisord -c $THIS_DIR/etc/supervisord.conf
	[ $? -ne 0 ] && { echo "start supervisord failed"; exit 1; }
	retry=0
	while [ $retry -lt 5 ]; do
		supervisorctl -c $THIS_DIR/etc/supervisord.conf status $PROGRAM |grep RUNNING >/dev/null
		[ $? -eq 0 ] && { break; }
		retry=$(($retry+1))
		sleep 1
	done
	[ $? -ge 5 ] && { echo "$PROGRAM server not in running status"; return 1; }
	return 0
}

function stop()
{
	supervisorctl -c $THIS_DIR/etc/supervisord.conf stop $PROGRAM >/dev/null 2>&1
	sleep 2
	supervisorctl -c $THIS_DIR/etc/supervisord.conf shutdown >/dev/null 2>&1
	sleep 2
	pid=$(ps axf|grep supervisord |grep $PROGRAM|awk '{print $1}')
	[ $? -eq 0 ] && { kill -9 $pid; }

	pid=$(ps axf|grep $PROGRAM |grep -v 'ps axf'|awk '{print $1}')
	[ $? -eq 0 ] && { kill -9 $pid; }
	return 0	
}

function restart()
{
	stop
	start
}

case $cmd in
	start)
		start
		;;
	stop)
		stop
		;;	
	restart)
		restart
		;;
	*)
		echo $"Usage: $0 {start|stop|restart}"
		RET=2	
esac
exit $RET

