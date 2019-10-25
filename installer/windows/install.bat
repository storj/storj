@echo off
rem uninstall existing storagenode product
msiexec /x {E97D368F-CB18-45B5-8799-1EBB10728A99}

rem NB: only installs from *first* release directory
for /d %%d in (release\*) do (
	msiexec /i %%d\storagenode_windows_amd64.msi /passive /qb /norestart /log %%d\install.log STORJ_WALLET="0x0000000000000000000000000000000000000000" STORJ_EMAIL="user@mail.example" STORJ_PUBLIC_ADDRESS="127.0.0.1:10000"
	goto :EOF
)
