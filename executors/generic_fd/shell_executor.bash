#!/bin/bash

cmd_EXEC() {
  local INDEX="$1"
  local FILE="$2"
}

while read INDEX CMD ARG0 ARG1 ARG2 ARG3 ARGX; do
  case "$CMD" in
    exec)
      cmd_EXEC "$INDEX" "$ARG0" "$ARG1" "$ARG2" "$ARG3"
      ;;
    
    *)
      echo ">> Unknown CMD"
      ;;
  esac
done < &3
