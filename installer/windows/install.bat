@echo off
setlocal enabledelayedexpansion

rem count # of args
set argC=0
for %%x in (%*) do Set /A argC+=1

if not %argC% gtr 0 (
    echo usage: %~nx0 ^<msi path^>
    exit /B 1
)

set msipath=%1
set props=STORJ_WALLET="0x0000000000000000000000000000000000000000" STORJ_EMAIL="user@mail.example" STORJ_PUBLIC_ADDRESS="127.0.0.1:10000"
for %%x in (%*) do (
    if  not %%x==%msipath% set props=!props! %%x
)

rem uninstall existing storagenode product
echo uninstalling storagenode
msiexec /uninstall %msipath%

echo installing storagenode from %msipath%
msiexec /i %msipath% /passive /qb /norestart /log %~dp1install.log %props%
