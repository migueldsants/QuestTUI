#!/usr/bin/env bash
# Records every README image with VHS (https://github.com/charmbracelet/vhs).
# Needs vhs, ttyd, ffmpeg and the JetBrainsMono Nerd Font installed.
#
#   docs/demo/record.sh            # all tapes
#   docs/demo/record.sh hero vigil # only these
set -euo pipefail
cd "$(dirname "$0")/../.."

go build -o quest .
go build -o /tmp/qt-seed docs/demo/seed.go
mkdir -p docs/images

tapes=("$@")
if [ ${#tapes[@]} -eq 0 ]; then
  tapes=(screens hero celebrations finale vigil notes themes light)
fi
for t in "${tapes[@]}"; do
  echo "recording $t"
  vhs "docs/demo/$t.tape"
done
ls -lh docs/images
