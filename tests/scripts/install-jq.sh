#!/bin/bash

OUTPUT=$1

if type -f jq; then
	exit 0
fi

case `uname` in
Linux)
	wget https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64 -O "$OUTPUT/jq"
	;;
Darwin)
	wget https://github.com/stedolan/jq/releases/download/jq-1.5/jq-osx-amd64 -O "$OUTPUT/jq"
	;;
*)
	echo "unsupported system!"
	exit 1
	;;
esac

chmod +x "$OUTPUT/jq"
