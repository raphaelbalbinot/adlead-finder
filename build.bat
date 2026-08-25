@echo off
echo ======================================================
echo    COMPILANDO NEXUS ADLEAD FINDER (GOLANG)
echo ======================================================
go mod tidy
go build -ldflags="-s -w" -o adlead-finder.exe ./cmd/server/main.go
if not exist .env (
    copy .env.example .env
    echo Arquivo .env criado! Configure suas chaves antes de rodar.
)
echo.
echo Compilacao concluida com sucesso: adlead-finder.exe gerado!
pause
