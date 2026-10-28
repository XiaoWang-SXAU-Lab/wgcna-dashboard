# Shared source, machine-local platform builds. See BUILDING.md.
RS_WGCNA ?= ../rs_WGCNA
.DEFAULT_GOAL := build
.PHONY: build windows macos worker frontend

ifeq ($(OS),Windows_NT)
build: windows
else
build: macos
endif

windows:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-windows.ps1 -WorkerSource "$(RS_WGCNA)"

macos:
	WGCNA_WORKER_SOURCE="$(RS_WGCNA)" bash scripts/build-macos.sh

# Compatibility aliases also build in isolation.
worker frontend: build
