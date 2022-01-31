@echo off
setlocal enabledelayedexpansion

rem NB: This script requires administrative privileges.
rem     It can't prompt for escalation if the `/q` option is used.

rem count # of args
set argC=0
for %%x in (%*) do Set /A argC+=1

if not %argC% gtr 0 (
    echo usage: %~nx0 ^[\q^] "<msi path (using '\' separators)>" ^[PROPERTY="value" ...^]
    exit /B 1
)

set interactivity=/passive /qb
if not %1==/q set msipath=%1
if %1==/q set msipath=%2
set props=STORJ_WALLET="0x0000000000000000000000000000000000000000" STORJ_EMAIL="user@mail.example" STORJ_PUBLIC_ADDRESS="127.0.0.1:10000"
for %%x in (%*) do (
    if  not %%x==%msipath% if not %%x==/q set props=!props! %%x
    if  %%x==/q set interactivity=/quiet /qn
)

rem uninstall existing storagenode product
echo uninstalling storagenode
call %~dp0uninstall.bat %msipath%

echo installing storagenode from %msipath%
msiexec /i %msipath% %interactivity% /norestart /log %~dp1install.log %props%
