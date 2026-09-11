@echo off
cd /d "%~dp0"
title AppSire CPE - Replicador Go (SUNAT)
echo =========================================================
echo    Iniciando Servidor de Descarga Masiva CPE SUNAT
echo =========================================================
echo Abriendo AutoSire en su propia ventana de escritorio...
timeout /t 1 /nobreak >nul
echo Ejecutando servidor...
appsire-server.exe
pause
