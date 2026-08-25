@echo off
if not exist adlead-finder.exe (
    echo Binario nao encontrado. Compilando pela primeira vez...
    call build.bat
)
echo Iniciando AdLead Finder...
start http://localhost:8080
adlead-finder.exe
