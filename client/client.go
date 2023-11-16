package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type HostAnel struct {
	IDHost string
	IPHost string
}

type Arquivos struct {
	NomeArquivo string
	Cliente     HostAnel
}

var ipNextNode string
var connNextNode net.Conn = nil

var ipPrevNode string
var connPrevNode net.Conn = nil

var privateKey string

var ipHost string

var ipSuperNo string

var connSuperNo net.Conn

var findServidorNext *Mensagem = nil
var findServidorPrev *Mensagem = nil

type Mensagem struct {
	Tipo       string
	IpOrigem   string
	IpDestino  string
	Conteudo   []byte
	IpAtual    string
	JumpsCount int
}

func splitMensagem(mensagem string) (*Mensagem, error) {
	mensagemSplit := strings.Split(mensagem, "#")

	if len(mensagemSplit) == 6 {
		jumps, _ := strconv.Atoi(mensagemSplit[5])
		return &Mensagem{mensagemSplit[0], mensagemSplit[1], mensagemSplit[2], []byte(mensagemSplit[3]), mensagemSplit[4], jumps}, nil
	} else {
		return nil, fmt.Errorf("mensagem inválida")
	}

}

func newMensagem(tipo string, IpOrigem string, IpDestino string, conteudo []byte, IpHost string, jumpsCount int) *Mensagem {
	return &Mensagem{tipo, IpOrigem, IpDestino, conteudo, IpHost, jumpsCount}
}

func newAck(ipDestino string) *Mensagem {
	return &Mensagem{"ack", ipHost, ipDestino, []byte(""), ipHost, 0}
}

func (m *Mensagem) copy() *Mensagem {
	return &Mensagem{m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, m.JumpsCount}
}

func (m *Mensagem) toString() string {
	return fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, m.JumpsCount)
}

func (m *Mensagem) toBytes() []byte {
	return []byte(m.toString())
}

func closeNextNode() {
	err := connNextNode.Close()
	errorHandler(err, "Erro ao fechar conexão com o próximo nó: ", false)

	connNextNode = nil
}

func closePrevNode() {
	err := connPrevNode.Close()
	errorHandler(err, "Erro ao fechar conexão com o anterior nó: ", false)

	connPrevNode = nil
}

func connectNextNode() net.Conn {
	conn, err := net.Dial("tcp", ipNextNode)
	errorHandler(err, "Erro ao conectar com o próximo nó: ", false)

	if err == nil {
		return conn
	}

	return nil
}

func connectPrevNode() net.Conn {
	conn, err := net.Dial("tcp", ipPrevNode)
	errorHandler(err, "Erro ao conectar com o anterior nó: ", false)

	if err == nil {
		return conn
	}

	return nil
}

func (m *Mensagem) sendNextNode() {
	if connNextNode == nil {
		connNextNode = connectNextNode()
	}

	var err error

	if connNextNode != nil {
		m.IpAtual = ipHost
		m.JumpsCount++

		_, err = connNextNode.Write(m.toBytes())

		if err != nil && errors.Is(err, syscall.EPIPE) {
			fmt.Println("Erro ao enviar mensagem para o proximo nó. Tentando novamente...")
			closeNextNode()
			time.Sleep(150 * time.Millisecond)
			m.sendNextNode()
		}
	}
}

func (m *Mensagem) sendPrevNode() {
	if connPrevNode == nil {
		connPrevNode = connectPrevNode()
	}

	if connPrevNode != nil {
		m.IpAtual = ipHost
		m.JumpsCount++

		_, err := connPrevNode.Write(m.toBytes())

		if err != nil && errors.Is(err, syscall.EPIPE) {
			fmt.Println("Erro ao enviar mensagem para o anterior nó. Tentando novamente...")
			closePrevNode()
			time.Sleep(150 * time.Millisecond)
			m.sendPrevNode()
		}
	}
}

func printIps() {
	fmt.Println("ipNextNo: ", ipNextNode)
	fmt.Println("ipPrevNo: ", ipPrevNode)
	fmt.Println("ipHost: ", ipHost)
}

func errorHandler(err error, msg string, fatal bool) {
	if fatal {
		if err != nil {
			log.Fatal(msg, err)
		}
	} else {
		if err != nil {
			log.Println(msg, err)
		}
	}
}

func openFileAndGetIps(filename string) []string {
	file, err := os.Open(filename)
	errorHandler(err, "Erro ao abrir arquivo: ", true)

	defer file.Close()

	scanner := bufio.NewScanner(file)

	var ips []string

	for scanner.Scan() {
		ips = append(ips, strings.TrimSpace(scanner.Text()))
	}

	return ips
}

var characterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characterRunes[rand.Intn(len(characterRunes))]
	}
	return string(b)
}

func newHashSha1(n ...int) string {
	noRandomCharacters := 32

	if len(n) > 0 {
		noRandomCharacters = n[0]
	}

	randString := RandomString(noRandomCharacters)

	hash := sha1.New()
	hash.Write([]byte(randString))
	bs := hash.Sum(nil)

	return fmt.Sprintf("%x", bs)
}

func getNextAndPrevAndHostManual(ipList []string, ipIndexFile int) (next string, prev string, host string) {
	if ipIndexFile == 0 {
		prev = ipList[len(ipList)-1]
		next = ipList[ipIndexFile+1]
	} else if ipIndexFile == len(ipList)-1 {
		prev = ipList[ipIndexFile-1]
		next = ipList[0]
	} else {
		prev = ipList[ipIndexFile-1]
		next = ipList[ipIndexFile+1]
	}
	host = ipList[ipIndexFile]

	return
}

func repassaMsg(msg *Mensagem) {
	// se não encontrou nenhuma mensagem válida, repassa para o próximo e anterior
	if msg.IpAtual == ipNextNode {
		msg.sendPrevNode()
	} else if msg.IpAtual == ipPrevNode {
		msg.sendNextNode()
	} else {
		// se não encontrou nenhuma mensagem válida, repassa para o próximo e anterior
		if msg.IpAtual == ipNextNode {
			msg.sendPrevNode()
		} else if msg.IpAtual == ipPrevNode {
			msg.sendNextNode()
		} else {
			// repassa pros dois lados
			cMsg := msg.copy()
			cMsg.sendNextNode()

			cMsg = msg.copy()
			cMsg.sendPrevNode()
		}
	}
}

func streamFile(conn net.Conn, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Erro ao abrir o arquivo:", err)
		return
	}
	defer file.Close()

	// Obtém o tamanho do arquivo
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Erro ao obter informações do arquivo:", err)
		return
	}
	fileSize := fileInfo.Size()

	fmt.Println("Enviando tamanho do arquivo...")
	binary.Write(conn, binary.LittleEndian, fileSize)

	time.Sleep(1 * time.Second)

	fmt.Println("Enviado bytes do arquivo...")
	n, err := io.CopyN(conn, file, fileSize)
	errorHandler(err, "Erro ao enviar arquivo:", false)

	if err == nil {
		fmt.Printf("Enviado %d bytes\n", n)
	}
}

// Receber conexoes da rede em anel
func receiveMessageAnelListening() {
	tcpAddrIpHost, err := net.ResolveTCPAddr("tcp", ipHost)

	// cria o servidor com o ip que está no arquivo
	tcpListener, err := net.ListenTCP("tcp", tcpAddrIpHost)
	defer tcpListener.Close()

	if err != nil {
		errorHandler(err, "Erro ao iniciar servidor TCP do anel: ", false)
		return
	}

	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			errorHandler(err, "Erro ao aceitar conexão TCP: ", false)
			continue
		}

		go func() { //leitura dos dados recebidos e tratamento deles
			defer conn.Close()
			for {
				buffer := make([]byte, 4000)

				msgLen, err := conn.Read(buffer)

				if err == io.EOF {
					fmt.Printf("[%s] A conexão foi fechada pelo nó.\n", conn.RemoteAddr().String())
					return
				}

				mensagem := make([]byte, msgLen)
				copy(mensagem, buffer[:msgLen]) // jeito mais seguro de copiar o buffer

				msg, err := splitMensagem(string(mensagem))
				if err != nil {
					errorHandler(err, "Erro ao converter mensagem: ", false)
					continue
				}

				//separa de quem veio a mensagem
				if msg.IpOrigem == ipHost || msg.JumpsCount > 20 {
					continue
				}

				if msg.IpDestino != ipHost && msg.IpDestino != "" {
					if msg.IpAtual == ipNextNode {
						msg.sendPrevNode()
					} else if msg.IpAtual == ipPrevNode {
						msg.sendNextNode()
					} else {
						// tunel
					}
				} else {
					switch msg.Tipo {
					case "findServidor":
						if msg.IpDestino == ipHost {
							if msg.IpAtual == ipNextNode {
								findServidorNext = msg
							} else if msg.IpAtual == ipPrevNode {
								findServidorPrev = msg
							}
						} else {
							repassaMsg(msg)
						}

					case "downloadFile":
						streamFile(conn, string(msg.Conteudo))
					default:
						repassaMsg(msg)
					}
				}

			}
		}()
	}
}

func getNextAndPrevAuto(ipList []string, host string) (string, string, string) {
	var next string
	var prev string
	for i, valor := range ipList { //busca o ip da maquina na lista de ips
		ipAtual, _, _ := net.SplitHostPort(valor)
		if strings.Contains(host, ipAtual) {
			if i == 0 {
				prev = ipList[len(ipList)-1]
				next = ipList[i+1]
			} else if i == len(ipList)-1 {
				prev = ipList[i-1]
				next = ipList[0]
			} else {
				prev = ipList[i-1]
				next = ipList[i+1]
			}
			host = valor
			break
		}
	}
	return next, prev, host
}

func getIpHost() (string, error) {
	addrs, err := net.InterfaceAddrs()
	errorHandler(err, "Erro ao obter endereços da interface: ", true)

	for _, address := range addrs {
		ipNet, ok := address.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("não foi possível obter o endereço IP da máquina")
}

func getFilesWithExtensionsInDir(dirPath string) []string {
	var files []string

	dir, err := os.Open(dirPath)
	if err != nil {
		fmt.Println("Erro ao abrir diretório:", err)
		return files
	}
	defer dir.Close()

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		fmt.Println("Erro ao ler diretório:", err)
		return files
	}

	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			// Verifica se o arquivo tem uma extensão
			if hasExtension(fileInfo.Name()) {
				files = append(files, fileInfo.Name())
			}
		}
	}

	return files
}

// Função auxiliar para verificar se um arquivo tem uma extensão
func hasExtension(filename string) bool {
	ext := filepath.Ext(filename)
	return ext != ""
}

func deleteFolder(folderPath string) error {
	err := os.RemoveAll(folderPath)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	cmd := exec.Command("clear") //Linux example, its tested
	cmd.Stdout = os.Stdout
	cmd.Run()

	deleteFolder("downloads")
	deleteFolder("seeders")
}

func main() {
	privateKey = newHashSha1()

	ipIndexFile := flag.Int("fi", -1, "Indice o arquivo de ips")
	//portaCliente := flag.String("p", "9980", "Porta do cliente")

	ipFile := flag.String("f", "ips", "Arquivo de ips")
	flag.Parse()

	listIp := openFileAndGetIps("../" + *ipFile)

	var err error
	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	// pega o ip da máquina sem a porta
	ipHost, err = getIpHost()
	errorHandler(err, "", true)

	if *ipIndexFile == -1 {
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAuto(listIp, ipHost)
	} else {
		// atribui a porta
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAndHostManual(listIp, *ipIndexFile)
	}

	// recebe mensagens do anel
	go receiveMessageAnelListening()

	menu()
}

func menu() {
	// upload arquivo, download arquivo, listar arquivos, buscar arquivo

	for {
		fmt.Println("1 - Upload arquivo")
		fmt.Println("2 - Download arquivo")
		fmt.Println("3 - Listar arquivos")
		fmt.Println("4 - Remover arquivo")
		fmt.Println("0 - Sair")
		fmt.Print("Opção: ")
		var opcao int
		fmt.Scan(&opcao)

		handleOpcoes(opcao)
	}

}

func resetChannels() {
	findServidorNext = nil
	findServidorPrev = nil
}

func findClosestServidor() net.Conn {
	defer resetChannels()

	msgNext := newMensagem("findServidor", ipHost, "", []byte(""), ipHost, 0)
	msgPrev := newMensagem("findServidor", ipHost, "", []byte(""), ipHost, 0)

	msgNext.sendNextNode()
	msgPrev.sendPrevNode()

	var conn net.Conn
	fmt.Println("Procurando servidores...")
	for {
		if findServidorNext != nil && findServidorPrev != nil {
			break
		}
	}

	ipJumpNext := strings.Split(string(findServidorNext.Conteudo), "/")
	ipJumpPrev := strings.Split(string(findServidorPrev.Conteudo), "/")

	if ipJumpNext[1] < ipJumpPrev[1] {
		conn, _ = net.Dial("tcp", ipJumpNext[0])
	} else {
		conn, _ = net.Dial("tcp", ipJumpPrev[0])
	}

	fmt.Println("Conexão estabelecida com o servidor: ", conn.RemoteAddr().String())

	return conn
}

func handleOpcoes(opcao int) {
	// aguarda ack
	buffer := make([]byte, 1024)

	connServidor := findClosestServidor()

	err := connServidor.SetReadDeadline(time.Now().Add(20 * time.Second))
	if err != nil {
		fmt.Println("Erro ao definir prazo de leitura:", err)
		return
	}

	defer connServidor.Close()
	switch opcao {
	case 1:
		arquivosDiretorio := getFilesWithExtensionsInDir("./")

		arquivosSeedering := getFilesWithExtensionsInDir("./seeders/" + ipHost)

		//excluir arquivos que já estão sendo seedados
		for _, arquivoSeedering := range arquivosSeedering {
			for i, arquivoDiretorio := range arquivosDiretorio {
				if arquivoSeedering == arquivoDiretorio {
					arquivosDiretorio = append(arquivosDiretorio[:i], arquivosDiretorio[i+1:]...)
					break
				}
			}
		}

		var arquivo string

		for {
			fmt.Println("Arquivos disponíveis para upload: ")
			for i, file := range arquivosDiretorio {
				fmt.Printf("\t%d - %s\n", i+1, file)
			}

			for i, file := range arquivosSeedering {
				fmt.Printf("\t%d - %s (seeding)\n", i+1+len(arquivosDiretorio), file)
			}
			fmt.Printf("\t0 - Voltar\n")

			fmt.Println()

			var opcao int
			fmt.Print("Opção: ")
			fmt.Scan(&opcao)

			if opcao >= 1 && opcao <= len(arquivosDiretorio) {
				arquivo = arquivosDiretorio[opcao-1]
				break
			} else if opcao >= len(arquivosDiretorio)+1 && opcao <= len(arquivosDiretorio)+len(arquivosSeedering) {
				fmt.Println("O arquivo já foi enviado para o servidor de arquivos!")
			} else if opcao == 0 {
				return
			} else {
				fmt.Printf("Opção inválida!\n\n")
			}
		}

		msg := newMensagem("uploadFile", ipHost, connServidor.RemoteAddr().String(), []byte(privateKey+"/"+arquivo), ipHost, 0)

		connServidor.Write(msg.toBytes())

		msgLen, err := connServidor.Read(buffer)

		if err != nil {
			// Verifique se o erro é devido ao prazo expirado
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("Tempo limite expirado. Problema com o servidor de arquivos.")
			} else {
				// Outro erro, imprima ou trate conforme necessário
				fmt.Println("Erro ao receber respostas da requisição:", err)
			}
			return
		}

		buffer = buffer[:msgLen]

		msg, err = splitMensagem(string(buffer))
		errorHandler(err, "", false)

		if err == nil {
			if msg.Tipo == "ack" {
				fmt.Println("Arquivo enviado com sucesso!")

				folderPath, err := createFolderIfNotExists("seeders")
				errorHandler(err, "Erro ao criar diretório de seeders: ", false)

				bytesToFile([]byte(privateKey), folderPath, arquivo)

			} else {
				fmt.Println("Erro ao enviar arquivo!")
			}
		}

	case 2:
		fmt.Println()
		var arquivo string

		fmt.Print("Nome do arquivo: ")
		fmt.Scan(&arquivo)

		arquivo = strings.TrimSpace(arquivo)

		msg := newMensagem("downloadFile", ipHost, connServidor.RemoteAddr().String(), []byte(arquivo), ipHost, 0)

		connServidor.Write(msg.toBytes())

		// aguarda ack
		msgLen, err := connServidor.Read(buffer)

		if err != nil {
			// Verifique se o erro é devido ao prazo expirado
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("Tempo limite expirado. Problema com o servidor de arquivos.")
			} else {
				// Outro erro, imprima ou trate conforme necessário
				fmt.Println("Erro ao receber respostas da requisição:", err)
			}
			return
		}

		buffer = buffer[:msgLen]

		msg, err = splitMensagem(string(buffer))
		errorHandler(err, "", false)

		if err == nil {
			var arq []byte = nil

			var tamArq int64

			if msg.Tipo == "MultiplePeers" {
				fmt.Println("Arquivo detectado em multiplos peers. Selecione um: ")
				peers := strings.Split(string(msg.Conteudo), "/")

				connClient, clientPeer := multiplePeers(peers)

				if connClient != nil {
					defer connClient.Close()

					msgDownload := newMensagem("downloadFile", ipHost, clientPeer, []byte(arquivo), ipHost, 0)

					fmt.Println("Enviando requisição de download...")
					connClient.Write(msgDownload.toBytes())

					arq, tamArq = receiveFile(connClient)
				}

			} else if msg.Tipo == "UniquePeer" {
				fmt.Println("Arquivo detectado em um único peer. Conectando com o peer ", string(msg.Conteudo), "...")

				connClient, clientPeer := uniquePeer(msg)

				msgDownload := newMensagem("downloadFile", ipHost, clientPeer, []byte(arquivo), ipHost, 0)

				fmt.Println("Enviando requisição de download...")
				connClient.Write(msgDownload.toBytes())

				arq, tamArq = receiveFile(connClient)

			} else if msg.Tipo == "AnotherNetworkPeer" {
				peers := strings.Split(string(msg.Conteudo), "/")

				if len(peers) > 1 {
					fmt.Println("Arquivo detectado em multiplos peers de outra rede. Selecione um: ")

					connClient, clientPeer := multiplePeers(peers)

					if connClient != nil {
						defer connClient.Close()

						msgDownload := newMensagem("downloadFile", ipHost, clientPeer, []byte(arquivo), ipHost, 0)

						fmt.Println("Enviando requisição de download...")
						connClient.Write(msgDownload.toBytes())

						arq, tamArq = receiveFile(connClient)
					}

				} else {
					fmt.Println("Arquivo detectado em um único peer de outra rede. Conectando com o peer ", strings.Split(string(msg.Conteudo), "-")[1], "...")

					connClient, clientPeer := uniquePeer(msg)

					defer connClient.Close()

					msgDownload := newMensagem("downloadFile", ipHost, clientPeer, []byte(arquivo), ipHost, 0)

					fmt.Println("Enviando requisição de download...")
					connClient.Write(msgDownload.toBytes())

					arq, tamArq = receiveFile(connClient)
				}

			} else {
				fmt.Println(msg.Tipo + ": " + string(msg.Conteudo))
				fmt.Println()
			}

			if arq != nil {
				downloadsPath, err := createFolderIfNotExists("downloads")
				errorHandler(err, "Erro ao criar diretório de downloads: ", false)

				fmt.Println("Arquivo recebido com sucesso, tamanho: ", tamArq)

				bytesToFile(arq, downloadsPath, arquivo)
			}
		}

	case 3:
		fmt.Println()
		msg := newMensagem("listFiles", ipHost, connServidor.RemoteAddr().String(), []byte(""), ipHost, 0)

		connServidor.Write(msg.toBytes())

		// aguarda ack
		msgLen, err := connServidor.Read(buffer)

		if err != nil {
			// Verifique se o erro é devido ao prazo expirado
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("Tempo limite expirado. Problema com o servidor de arquivos.")
			} else {
				// Outro erro, imprima ou trate conforme necessário
				fmt.Println("Erro ao receber respostas da requisição:", err)
			}
			return
		}

		errorHandler(err, "Erro ao receber respostas da requisição: ", false)
		buffer = buffer[:msgLen]

		msg, err = splitMensagem(string(buffer))
		errorHandler(err, "", false)

		if err == nil {
			if msg.Tipo == "ack" {
				// listar arquivos
				var tabelaArquivos []Arquivos

				err = json.Unmarshal(msg.Conteudo, &tabelaArquivos)
				errorHandler(err, "Erro ao desconverter tabela de arquivos: ", false)

				if err == nil {
					fmt.Printf("************* Arquivos disponiveis *************\n")
					peerFlag := false
					var lastPeer string
					for _, tabelaArquivo := range tabelaArquivos {
						if lastPeer != tabelaArquivo.Cliente.IPHost && peerFlag {
							peerFlag = false
							fmt.Println()
						}

						if !peerFlag {
							fmt.Printf("Peer: %s\n", tabelaArquivo.Cliente.IPHost)
							peerFlag = true
						}
						fmt.Printf("\t %s\n", tabelaArquivo.NomeArquivo)
						lastPeer = tabelaArquivo.Cliente.IPHost
					}
					fmt.Printf("\n\n")
				}
			} else {
				fmt.Println(msg.Tipo + ": " + string(msg.Conteudo))
				fmt.Println()
			}
		}
	case 4:
		arquivosSeedering := getFilesWithExtensionsInDir("./seeders/" + ipHost)

		var arquivo string

		for {
			fmt.Println("Arquivos disponíveis para remover: ")
			for i, file := range arquivosSeedering {
				fmt.Printf("\t%d - %s\n", i+1, file)
			}
			fmt.Printf("\t0 - Voltar\n")

			fmt.Println()

			var opcao int
			fmt.Print("Opção: ")
			fmt.Scan(&opcao)

			if opcao >= 1 && opcao <= len(arquivosSeedering) {
				arquivo = arquivosSeedering[opcao-1]
				break
			} else if opcao == 0 {
				return
			} else {
				fmt.Printf("Opção inválida!\n\n")
			}
		}

		msg := newMensagem("removeFile", ipHost, connServidor.RemoteAddr().String(), []byte(privateKey+"/"+arquivo), ipHost, 0)

		connServidor.Write(msg.toBytes())

		msgLen, err := connServidor.Read(buffer)

		if err != nil {
			// Verifique se o erro é devido ao prazo expirado
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("Tempo limite expirado. Problema com o servidor de arquivos.")
			} else {
				// Outro erro, imprima ou trate conforme necessário
				fmt.Println("Erro ao receber respostas da requisição:", err)
			}
			return
		}

		buffer = buffer[:msgLen]

		msg, err = splitMensagem(string(buffer))
		errorHandler(err, "", false)

		if err == nil {
			if msg.Tipo == "ack" {
				fmt.Println("Arquivo removido com sucesso!")

				filePath := fmt.Sprintf("./seeders/%s/%s", ipHost, arquivo)

				err = deleteFileFromFolder(filePath)
			} else {
				fmt.Println("Erro ao enviar arquivo!")
			}
		}
	case 0:
		fmt.Println("Sair")

	default:
		fmt.Println("Opção inválida!")
	}

}

func uniquePeer(msg *Mensagem) (net.Conn, string) {
	peerIp := strings.Split(string(msg.Conteudo), "-")[1]
	clientPeer := peerIp

	connClient, err := net.Dial("tcp", clientPeer)
	errorHandler(err, "Erro ao conectar com o cliente: ", false)

	return connClient, clientPeer
}

func multiplePeers(peers []string) (net.Conn, string) {
	for i, peer := range peers {
		peerIp := strings.Split(peer, "-")
		fmt.Printf("\t%d - %s\n", i+1, peerIp[1])
	}
	fmt.Printf("\t0 - Voltar\n")

	var opcao int

	var peerIp string

	for {
		fmt.Print("Opção: ")
		fmt.Scan(&opcao)

		if opcao >= 1 && opcao <= len(peers) {
			// abre conexão direta com o peer
			peerIp = strings.Split(peers[opcao-1], "-")[1]
			fmt.Println("Conectando com o peer ", peerIp, "...")

			break

		} else if opcao == 0 {
			return nil, ""
		} else {
			fmt.Printf("Opção inválida!\n\n")
		}
	}

	clientPeer := peerIp

	connClient, err := net.Dial("tcp", clientPeer)
	errorHandler(err, "Erro ao conectar com o cliente: ", false)

	return connClient, clientPeer
}

func receiveFile(conn net.Conn) ([]byte, int64) {
	var size int64

	arquivoBytes := new(bytes.Buffer)

	fmt.Println("Recebendo tamanho do arquivo...")
	binary.Read(conn, binary.LittleEndian, &size)

	time.Sleep(1 * time.Second)

	fmt.Println("Recebendo bytes do arquivo...")
	n, err := io.CopyN(arquivoBytes, conn, size)
	if err != nil {
		fmt.Println("Erro ao receber arquivo:", err)
		return nil, 0
	}

	return arquivoBytes.Bytes(), n
}

func bytesToFile(data []byte, folderPath, filename string) error {
	filePath := fmt.Sprintf("%s/%s", folderPath, filename)

	// Cria ou abre um arquivo para escrita
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Escreve os bytes no arquivo
	_, err = file.Write(data)
	if err != nil {
		return err
	}

	fmt.Println("Arquivo criado com sucesso:", filePath)
	return nil
}

func createFolderIfNotExists(folderName string) (string, error) {
	folderPath := fmt.Sprintf("./%s/%s", folderName, ipHost)

	// Verifica se o diretório já existe
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		// Se não existir, cria o diretório
		err := os.MkdirAll(folderPath, 0755) // Permissões 0755 para o diretório
		if err != nil {
			return "", err
		}
	}

	return folderPath, nil
}

func deleteFileFromFolder(filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		return err
	}

	fmt.Println("Arquivo", filePath, "do IPHost", ipHost, "removido com sucesso.")
	return nil
}
