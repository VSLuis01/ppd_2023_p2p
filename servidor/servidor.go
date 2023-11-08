package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var tabelasDeRoteamento []string

var ipNextNode string
var connNextNode *net.TCPConn = nil

var ipPrevNode string
var connPrevNode *net.TCPConn = nil

var privateKey string

var ipHost string

type Mensagem struct {
	Tipo       string
	IpOrigem   string
	IpDestino  string
	Conteudo   []byte
	IpAtual    string
	JumpsCount int
}

func splitMensagem(mensagem string) Mensagem {
	mensagemSplit := strings.Split(mensagem, "#")

	jumps, _ := strconv.Atoi(mensagemSplit[5])

	return Mensagem{mensagemSplit[0], mensagemSplit[1], mensagemSplit[2], []byte(mensagemSplit[3]), mensagemSplit[4], jumps}
}

func newMensagem(tipo string, IpOrigem string, IpDestino string, conteudo []byte, IpHost string, jumpsCount int) Mensagem {
	return Mensagem{tipo, IpOrigem, IpDestino, conteudo, IpHost, jumpsCount}
}

func (m *Mensagem) toString() string {
	return fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, m.JumpsCount)
}

func (m *Mensagem) toBytes() []byte {
	return []byte(m.toString())
}

func connectNextNode(conn *net.TCPConn) {
	tcpAddrHost, _ := net.ResolveTCPAddr("tcp", ipHost)
	tcpAddrNextNode, _ := net.ResolveTCPAddr("tcp", ipNextNode)

	conn, err := net.DialTCP("tcp", tcpAddrHost, tcpAddrNextNode)
	errorHandler(err, "Erro ao conectar com o próximo nó: ", false)

	defer func(conn *net.TCPConn) {
		err := conn.Close()
		errorHandler(err, "Erro ao fechar conexão com o próximo nó: ", false)
	}(conn)
}

func connectPrevNode(conn *net.TCPConn) {
	tcpAddrHost, _ := net.ResolveTCPAddr("tcp", ipHost)
	tcpAddrPrevNode, _ := net.ResolveTCPAddr("tcp", ipPrevNode)

	conn, err := net.DialTCP("tcp", tcpAddrHost, tcpAddrPrevNode)
	errorHandler(err, "Erro ao conectar com o anterior nó: ", false)

	defer func(conn *net.TCPConn) {
		err := conn.Close()
		errorHandler(err, "Erro ao fechar conexão com o anterior nó: ", false)
	}(conn)
}

func (m *Mensagem) sendNextNode() error {
	if connNextNode == nil {
		connectNextNode(connNextNode)
	}

	mensagem := fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, 0)

	_, err := connNextNode.Write([]byte(mensagem))

	return err
}

func (m *Mensagem) sendPrevNode() error {
	if connPrevNode == nil {
		connectPrevNode(connPrevNode)
	}

	mensagem := fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, 0)

	_, err := connPrevNode.Write([]byte(mensagem))

	return err
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

// Receber conexoes da rede em anel
func receiveMessageAnelListening() {
	tcpAddrIpHost, err := net.ResolveTCPAddr("tcp", ipHost)

	tcpListener, err := net.ListenTCP("tcp", tcpAddrIpHost)

	if err != nil {
		errorHandler(err, "Erro ao iniciar servidor TCP do anel: ", false)
		return
	}

	for {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão TCP: ", false)

		go func() { //leitura dos dados recebidos e tratamento deles
			for {
				buffer := make([]byte, 4000)

				msgLen, err := conn.Read(buffer)
				errorHandler(err, "Erro ao ler mensagem TCP: ", false)

				mensagem := make([]byte, msgLen)
				copy(mensagem, buffer[:msgLen]) // jeito mais seguro de copiar o buffer

				msg := splitMensagem(string(mensagem))

				//separa de quem veio a mensagem
				if msg.IpAtual == ipNextNode {

					// aqui vai a logica de tratamento da mensagem (broadcast, etc)

					fmt.Println("Mensagem recebida do nó seguinte: ", msg.toString())
				} else {

					// aqui vai a logica de tratamento da mensagem (broadcast, etc)

					fmt.Println("Mensagem recebida do nó anterior: ", msg.toString())
				}

			}
		}()
	}
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

// Conexões diretas com o nó servidor
func tcpHandleMessages(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		errorHandler(err, "Erro ao fechar conexão TCP: ", true)
	}(conn)

	msg := newMensagem("chave", ipHost, conn.RemoteAddr().String(), []byte(privateKey), ipHost, 0)

	_, err := conn.Write(msg.toBytes())
	errorHandler(err, "Erro ao enviar chave para o super nó: ", false)
	fmt.Println("Chave enviada para o super nó")

	for {
		buffer := make([]byte, 4000)
		msgLen, err := conn.Read(buffer)

		if err == io.EOF {
			fmt.Printf("[%s] A conexão foi fechada pelo nó.\n", conn.RemoteAddr().String())
			return
		}

		errorHandler(err, "Erro ao ler mensagem TCP: ", false)

		mensagem := make([]byte, msgLen)
		copy(mensagem, buffer[:msgLen])

		msg = splitMensagem(string(mensagem))

		fmt.Printf("[%s] Enviou: %s - Tipo > %s\n", conn.RemoteAddr().String(), string(msg.Conteudo), msg.Tipo)

		if strings.EqualFold(msg.Tipo, "ack") {
			fmt.Println("Chave recebida pelo super nó")

			// Solicita a tabela dos super nós
			msg = newMensagem("roteamento-supers", ipHost, conn.RemoteAddr().String(), []byte(""), ipHost, 0)

			_, err = conn.Write(msg.toBytes())
			errorHandler(err, "Erro ao solicitar tabela de roteamento dos super nós: ", false)

			// Recebe a tabela de roteamento dos super nós
			buffer = make([]byte, 1024)
			msgLen, err = conn.Read(buffer)

			buffer = buffer[:msgLen]
			msg = splitMensagem(string(buffer))

			fmt.Println("Tabela de roteamento recebida do super nó: ")
			err = json.Unmarshal(msg.Conteudo, &tabelasDeRoteamento)
			errorHandler(err, "Erro ao converter tabela de roteamento:", true)

			for _, supers := range tabelasDeRoteamento {
				fmt.Println(supers)
			}
			continue
		}
	}
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

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {
	//ctx := context.Background()

	ipIndexFile := flag.Int("fi", -1, "Indice o arquivo de ips")
	portaSuperNo := flag.String("ps", "8001", "Porta do super nó")
	flag.Parse()

	listIp := openFileAndGetIps("../ips")

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	var err error
	ipHost, err = getIpHost()
	errorHandler(err, "Erro ao obter endereço IP da máquina: ", true)

	if *ipIndexFile == -1 {
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAuto(listIp, ipHost)
	} else {
		// atribui a porta
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAndHostManual(listIp, *ipIndexFile)
	}

	//go receiveMessageAnelListening(ipHost)

	fmt.Println("Se conectando a um super nó...")

	//por padrao o endereço do superno de configuração inicial é o segundo elemento da lista de ips
	ipSuperNo, _, _ := net.SplitHostPort(listIp[1])

	ipSuperNo = ipSuperNo + ":" + *portaSuperNo

	tcpAddrSuperNo, _ := net.ResolveTCPAddr("tcp", ipSuperNo)
	tcpAddrIpHost, _ := net.ResolveTCPAddr("tcp", ipHost)

	conn, err := net.DialTCP("tcp", tcpAddrIpHost, tcpAddrSuperNo)
	errorHandler(err, "Erro ao conectar ao super nó:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")

	privateKey = newHashSha1()

	go tcpHandleMessages(conn)

	// recebe mensagens do anel
	go receiveMessageAnelListening()

	// Wait forever
	select {}
}
