# Nome dos executáveis
EXECUTABLE_MASTER = bin/master
EXECUTABLE_SUPER = bin/super
EXECUTABLE_SERVIDOR = bin/servidor

# Lista de arquivos Go
SOURCES = master.go super.go servidor.go

# Comando de compilação Go
GO = go

# Comando para remover arquivos
RM = rm -rf

# Diretório para os arquivos binários
BIN_DIR = bin

# Comando padrão (all)
all: build

# Compilação
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(EXECUTABLE_MASTER) master.go
	$(GO) build -o $(EXECUTABLE_SUPER) super.go
	$(GO) build -o $(EXECUTABLE_SERVIDOR) servidor.go

# Limpeza dos arquivos gerados
clean:
	$(RM) $(BIN_DIR)

.PHONY: all build clean