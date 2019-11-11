@echo off

rem count # of args
set argC=0
for %%x in (%*) do Set /A argC+=1

if not "%argC%"=="1" (
    echo usage: %~nx0 ^<msi path^>
    exit /B 1
)

rem uninstall existing storagenode product
echo uninstalling storagenode
msiexec /uninstall %1

echo installing storagenode from %1
msiexec /i %1 /passive /qb /norestart /log %~dp1install.log STORJ_WALLET="0x0000000000000000000000000000000000000000" STORJ_EMAIL="user@mail.example" STORJ_PUBLIC_ADDRESS="127.0.0.1:10000"
