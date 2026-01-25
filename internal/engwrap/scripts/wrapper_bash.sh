#!/bin/bash
export PROMPT_COMMAND='history -a; LASTCMD=$(history 1 | sed "s/^[ ]*[0-9]*[ ]*//"); TIMESTAMP=$(date +%Y-%m-%d_%H:%M:%S | tr "_" " "); echo "$TIMESTAMP - $LASTCMD" >> {{.CmdLogPath}}'
exec bash
