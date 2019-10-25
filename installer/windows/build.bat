@echo off
for /d %%d in (release\*) do (
    rem copy the storagenode binaries to the installer project
    copy %%d\storagenode_windows_amd64.exe installer\windows\storagenode.exe
    copy %%d\storagenode-updater_windows_amd64.exe installer\windows\storagenode-updater.exe

    rem build the installer
    msbuild installer\windows\windows.sln /t:Build /p:Configuration=Release

    rem copy the MSI to the release dir
    copy installer\windows\bin\Release\storagenode.msi %%d\storagenode_windows_amd64.msi
)
