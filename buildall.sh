#!/bin/bash
source /home/flo/git/golang-crosscompile/crosscompile.bash;
#go-linux-amd64 build -o bin/linux_amd64/hync main.go;
go-windows-386 build -o bin/windows_386/hync.exe main.go;
#go-windows-amd64 build -o bin/windows_amd64/hync.exe main.go;
