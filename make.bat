@echo off

REM Verifica se o primeiro argumento é "clean"
IF "%1"=="clean" (
    echo Limpando arquivos gerados...
    rmdir /s /q bin
    exit /b
)

REM Lista de diretórios com seus respectivos go.mod
set MODULES=mestre superno servidor

REM Loop para compilar cada módulo
for %%i in (%MODULES%) do (
    echo Compilando o módulo %%i...
    cd %%i
    go build -o ..\bin\%%i
    cd ..
)

echo Compilação concluída!
