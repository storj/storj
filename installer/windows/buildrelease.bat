@echo off

rem build msi installer for each release directory
for /d %%d in (release\*) do (
    call %~dp0build.bat %%d\storagenode_windows_amd64.exe %%d\storagenode-updater_windows_amd64.exe %%d\storagenode_windows_amd64.msi
)
