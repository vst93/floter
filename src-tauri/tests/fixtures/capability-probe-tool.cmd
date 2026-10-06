@echo off
if "%1"=="--version" (echo floter-tool 1.2.3 & exit /b 0)
if "%1"=="--help" (echo Usage: floter-tool [options] & exit /b 0)
if "%1"=="--features" (echo json markdown & exit /b 0)
if "%1"=="--defunct" (echo not supported & exit /b 3)
echo unknown flag: %1 1>&2
exit /b 1
