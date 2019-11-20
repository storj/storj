@echo off

rem count # of args
set argC=0
for %%x in (%*) do Set /A argC+=1

if not "%argC%" == "1" (
    echo usage: %~nx0 ^<msi path^>
    exit /B 1
)

set msipath=%1
msiexec /uninstall %msipath%
