package main

import (
	"bufio"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

var tabelaArquivos []Arquivos

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
	if connNextNode != nil {
		err := connNextNode.Close()
		errorHandler(err, "Erro ao fechar conexão com o próximo nó: ", false)
	}
}

func closePrevNode() {
	if connPrevNode != nil {
		err := connPrevNode.Close()
		errorHandler(err, "Erro ao fechar conexão com o anterior nó: ", false)
	}
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
		errorHandler(err, "Erro ao enviar mensagem para o próximo nó: ", false)
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
		errorHandler(err, "Erro ao enviar mensagem para o anterior nó: ", false)
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

	fmt.Println("Aguardando conexões do anel...")
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
				if msg.IpOrigem == ipHost {
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
						if msg.IpAtual == ipNextNode {
							findServidorNext = msg
						} else if msg.IpAtual == ipPrevNode {
							findServidorPrev = msg
						}
					default:
						// se não encontrou nenhuma mensagem válida, repassa para o próximo e anterior
						if msg.IpAtual == ipNextNode {
							fmt.Println("Repassando mensagem para o nó anterior...")
							msg.sendPrevNode()
						} else if msg.IpAtual == ipPrevNode {
							fmt.Println("Repassando mensagem para o próximo nó...")
							msg.sendNextNode()
						} else {
							// repassa pros dois lados
							fmt.Println("Repassando mensagem para o próximo e anterior nó...")
							cMsg := msg.copy()
							cMsg.sendNextNode()

							cMsg = msg.copy()
							cMsg.sendPrevNode()
						}
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

func findServidor(someIp string) net.Conn {
	conn, err := net.Dial("tcp", someIp)
	errorHandler(err, "Erro ao conectar com algum nó do anel: ", true)

	conn.Write(newMensagem("FindSuper", ipHost, "", []byte(""), ipHost, 0).toBytes())

	listener, err := net.Listen("tcp", ipHost)
	errorHandler(err, "Erro ao abrir porta para receber resposta do supernó: ", true)

	connSuper, err := listener.Accept()
	errorHandler(err, "Erro ao receber resposta do supernó: ", true)

	return connSuper
}

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	cmd := exec.Command("clear") //Linux example, its tested
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	privateKey = newHashSha1()

	ipIndexFile := flag.Int("fi", -1, "Indice o arquivo de ips")
	//portaCliente := flag.String("p", "9980", "Porta do cliente")
	flag.Parse()

	listIp := openFileAndGetIps("../ips")

	var err error
	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	// pega o ip da máquina sem a porta
	ipHost, err = getIpHost()
	errorHandler(err, "", true)

	fmt.Println(ipHost)

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
		fmt.Println("4 - Buscar arquivo")
		fmt.Println("5 - Sair")
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

	//connServidor := findClosestServidor()

	switch opcao {
	case 1:
		fmt.Println("Upload arquivo")
	case 2:
		fmt.Println("Download arquivo")
	case 3:
		fmt.Println("Listar arquivos")
	case 4:
		fmt.Println("Buscar arquivo")
	case 5:
		fmt.Println("Sair")
	}

}
