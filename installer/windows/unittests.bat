rem install NuGet packages
nuget install installer\windows\StorjTests\packages.config -o installer\windows\packages

rem build the test project
msbuild installer\windows\StorjTests

rem run the unit tests
vstest.console StorjTests\bin\Debug\StorjTests.dll
