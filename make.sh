#!/bin/bash

# Verifica se o primeiro argumento é "clean"
if [ "$1" == "clean" ]; then
    echo "Limpando arquivos gerados..."
    rm -rf bin
    exit 0
fi

# Lista de diretórios com seus respectivos go.mod
MODULES=("mestre" "superno" "servidor")

# Loop para compilar cada módulo
for module in "${MODULES[@]}"
do
    echo "Compilando o módulo $module..."
    cd $module
    go build -o ../bin/$module
    cd ..
done

echo "Compilação concluída!"
