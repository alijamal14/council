@echo off
SET SCRIPT_DIR=%~dp0
SET BINARY=%SCRIPT_DIR%council.exe

IF NOT EXIST "%BINARY%" (
    echo 🔧 Council binary not found. Building...
    pushd "%SCRIPT_DIR%"
    go build -o council.exe .
    popd
)

"%BINARY%" %*
