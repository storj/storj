@echo off
if "%1"=="" (
    echo "missing required msi path argument"
    exit /B 1
)

rem uninstall existing storagenode product
echo uninstalling storagenode
msiexec /x {E97D368F-CB18-45B5-8799-1EBB10728A99} /passive /qb

echo installing storagenode from %1
msiexec /i %1 /passive /qb /norestart /log %~dp1install.log STORJ_WALLET="0x0000000000000000000000000000000000000000" STORJ_EMAIL="user@mail.example" STORJ_PUBLIC_ADDRESS="127.0.0.1:10000"
