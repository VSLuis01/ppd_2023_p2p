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
	"strings"
	"time"
)

var tabelaRoteamentoSuperNos []net.Conn

var ipNextNode string
var connNextNode *net.TCPConn = nil
var ipPrevNode string
var connPrevNode *net.TCPConn = nil
var ipHost string
var privateKey string

type Mensagem struct {
	Tipo       string
	IpOrigem   string
	IpDestino  string
	Conteudo   string
	IpHost     string
	JumpsCount int
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

	mensagem := fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpHost, 0)

	_, err := fmt.Fprintf(connNextNode, mensagem)
	return err
}

func (m *Mensagem) sendPrevNode() error {
	if connPrevNode == nil {
		connectPrevNode(connPrevNode)
	}

	mensagem := fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpHost, 0)

	_, err := fmt.Fprintf(connPrevNode, mensagem)
	return err
}

func printIps() {
	fmt.Println("ipNextNode: ", ipNextNode)
	fmt.Println("ipPrevNode: ", ipPrevNode)
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

func tcpHandleMessages(conn net.Conn, ackChan chan<- bool) {
	defer func(conn net.Conn) {
		err := conn.Close()
		errorHandler(err, "Erro ao fechar conexão TCP: ", true)
	}(conn)

	for {
		buffer := make([]byte, 4000)
		msgLen, err := conn.Read(buffer)

		if err == io.EOF {
			fmt.Printf("[%s] A conexão foi fechada pelo nó.\n", conn.RemoteAddr().String())
			err = conn.Close()
			errorHandler(err, fmt.Sprintf("Erro ao fechar conexão com [%s]", conn.RemoteAddr().String()), false)
			return
		}

		errorHandler(err, "Erro ao ler mensagem TCP: ", false)

		mensagem := make([]byte, msgLen)
		copy(mensagem, buffer[:msgLen])

		fmt.Printf("[%s] Enviou: %s\n", conn.RemoteAddr().String(), string(mensagem))

		if strings.EqualFold(string(mensagem), "ack") {
			ackChan <- true
			continue
		} else if strings.EqualFold(string(mensagem), "roteamento-supers") {
			// slice dos ips dos supernos
			var ips []string

			for _, super := range tabelaRoteamentoSuperNos {
				ips = append(ips, super.RemoteAddr().String())
			}

			bytesRoteamento, err := json.Marshal(ips)
			errorHandler(err, "Erro ao serializar tabela de roteamento: ", false)

			_, err = conn.Write(bytesRoteamento)
			errorHandler(err, "Erro ao enviar tabela de roteamento: ", false)

			continue
		}
	}
}

/*func receiveMessageAnelListening(adress string) {
	tcpListener, err := net.Listen("tcp", adress)
	if err != nil {
		errorHandler(err, "Erro ao ler mensagem TCP: ", false)
		return
	}
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			errorHandler(err, "Erro ao ler mensagem TCP: ", false)
			return
		}
		go func() { //leitura dos dados recebidos e tratamento deles
			//ipTratamento, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			fmt.Println("Conexão TCP estabelecida com sucesso:", conn.RemoteAddr().String())
			for {
				buffer := make([]byte, 1024)
				n, err := conn.Read(buffer)
				//	decodedMessage, err := base64.StdEncoding.DecodeString(string(buffer[:n]))
				recebimento := string(buffer[:n])
				aux := strings.Split(recebimento, "#")
				if len(aux) < 5 { //evita mensagens com formato invalido
					continue
				}
				fmt.Println("Mensagem recebida: [", aux, "],[", recebimento, "]")
				m := mensagem{IpOrigem: aux[1], IpDestino: aux[2], Conteudo: aux[3], IpAtual: aux[4]}
				fmt.Println("mensagem criada")
				tipo := aux[0]
				if err != nil {
					errorHandler(err, "Erro ao ler mensagem TCP: ", false)
					return
				}
				//separa de quem veio a mensagem
				if m.IpAtual == ipNextNode {
					fmt.Println("Mensagem recebida do nó proximo: ", m.Conteudo, " tipo: ", tipo)
					sendMessageNext(buffer[:n])
				} else {
					fmt.Println("Mensagem recebida do nó anterior: ")
				}

			}
		}()
	}
}*/

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

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {

	//ctx := context.Background()

	// definindo a porta do nó mestre
	ipIndexFile := flag.Int("fi", -1, "Porta destino")
	flag.Parse()

	listIp := openFileAndGetIps("../ips")

	// pega o ip da máquina sem a porta
	ipHost, err := getIpHost()
	errorHandler(err, "", true)

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	if *ipIndexFile == -1 {
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAuto(listIp, ipHost)
	} else {
		// atribui a porta
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAndHostManual(listIp, *ipIndexFile)
	}

	//inicia recepçao de mensagens do anel
	/*go receiveMessageAnelListening(ipHost) */

	// servidor tcp
	tcpListener, err := net.Listen("tcp", ipHost)
	errorHandler(err, "Erro ao criar servidor TCP: ", true)

	defer func(tcpListener net.Listener) {
		err := tcpListener.Close()
		errorHandler(err, "Erro ao fechar servidor TCP: ", true)
	}(tcpListener)

	// cria uma chave única SHA1
	privateKey = newHashSha1()

	// lida com conexoes de outros supernós
	fmt.Println("Aguardando supernós se conectarem...")

	// canais criados para controlar a conexão dos supernós
	ackChan := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão: ", false)

		fmt.Println("O super nó", i+1, " se conectou com sucesso!")
		tabelaRoteamentoSuperNos = append(tabelaRoteamentoSuperNos, conn)

		// envia a chave de identificação unica do nó mestre
		_, err = conn.Write([]byte(privateKey))
		errorHandler(err, "Erro ao enviar a chave de identificação", false)

		go tcpHandleMessages(conn, ackChan)
	}

	// Aguardar que ambos os nós se conectem
	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os super nós se conectaram com sucesso!")

		// envia confirmacao aos supernós
		for _, conn := range tabelaRoteamentoSuperNos {
			_, err = conn.Write([]byte("ok"))
			errorHandler(err, "Erro ao enviar ok", false)
		}
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	select {}

}
