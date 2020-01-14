@echo off

rem NB: only installs from *first* release directory
for /d %%d in (release\*) do (
    call %~dp0install.bat %%d\storagenode_windows_amd64.msi
	goto :EOF
)
