#!/bin/bash

server="./projectc-ethereum-connector"
let item=0
item=`ps -ef | grep $server | grep -v grep | wc -l`

if [ $item -eq 1 ]; then
	echo "The projectc-ethereum-connector is running, shut it down..."
	pid=`ps -ef | grep $server | grep -v grep | awk '{print $2}'`
	kill -9 $pid
fi

echo "Start projectc-ethereum-connector now ..."
make build
./build/pkg/cmd/projectc-ethereum-connector/projectc-ethereum-connector  >> projectc-ethereum-connector.log 2>&1 &
