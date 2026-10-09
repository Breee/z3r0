#!/bin/sh
# Usage: scripts/sandbox-down.sh <name>
set -eu
openshell sandbox delete "$1"
