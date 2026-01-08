@echo off

rem build msi installer for each release directory
for /d %%d in (release\*) do (
    call %~dp0build.bat %%d\windows_amd64\storagenode.exe %%d\windows_amd64\storagenode-updater.exe %%d\windows_amd64\storagenode.msi
)
