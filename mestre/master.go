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
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var tabelaRoteamentoSuperNos []string

var ipNextNode string
var connNextNode net.Conn = nil

var ipPrevNode string
var connPrevNode net.Conn = nil

var ipHost string

var privateKey string

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

	return conn
}

func connectPrevNode() net.Conn {
	conn, err := net.Dial("tcp", ipPrevNode)
	errorHandler(err, "Erro ao conectar com o anterior nó: ", false)

	return conn
}

func (m *Mensagem) sendNextNode() error {
	if connNextNode == nil {
		connNextNode = connectNextNode()
	}

	m.IpAtual = ipHost
	m.JumpsCount++

	_, err := connNextNode.Write(m.toBytes())

	return err
}

func (m *Mensagem) sendPrevNode() error {
	if connPrevNode == nil {
		connPrevNode = connectPrevNode()
	}

	m.IpAtual = ipHost
	m.JumpsCount++

	_, err := connPrevNode.Write(m.toBytes())

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

// Essa função é responsável para tratar de conexões diretas com o nó mestre
func tcpHandleMessages(conn net.Conn, ackChan chan<- bool, finishChan chan<- bool) {
	defer func(conn net.Conn) {
		err := conn.Close()
		errorHandler(err, "Erro ao fechar conexão TCP: ", true)
	}(conn)

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

		msg, _ := splitMensagem(string(mensagem))

		fmt.Printf("[%s] Enviou: %s - Tipo > %s\n", conn.RemoteAddr().String(), string(msg.Conteudo), msg.Tipo)

		if strings.EqualFold(msg.Tipo, "ack") {
			ackChan <- true

			tabelaRoteamentoSuperNos = append(tabelaRoteamentoSuperNos, string(msg.Conteudo))
			continue
		} else if strings.EqualFold(msg.Tipo, "roteamento-supers") {

			bytesRoteamento, err := json.Marshal(tabelaRoteamentoSuperNos)
			errorHandler(err, "Erro ao serializar tabela de roteamento: ", false)

			msg = newMensagem("roteamento-supers", ipHost, conn.RemoteAddr().String(), bytesRoteamento, ipHost, 0)

			_, err = conn.Write(msg.toBytes())
			errorHandler(err, "Erro ao enviar tabela de roteamento: ", false)

			continue
		} else if strings.EqualFold(msg.Tipo, "ok") {
			finishChan <- true
			continue
		}
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
				if msg.IpAtual == ipNextNode {
					// aqui vai a logica de tratamento da mensagem (broadcast, etc)

					if msg.IpDestino == ipHost { // usa a mensagem recebida
						if msg.JumpsCount > 6 {
							fmt.Println("Mensagem descartada por ter ultrapassado o limite de saltos")
							fmt.Println(msg.toString())
							continue
						} else {
							msg.sendNextNode()
						}
					} else { // avalia o tipo e repassa a mensagem
						switch msg.Tipo {
						case "next": // repassa a mensagem
							fmt.Println("Repassado a mensagem para o próximo nó")

							msg.sendNextNode()
						default:

						}
					}

				} else if msg.IpAtual == ipPrevNode {
					// aqui vai a logica de tratamento da mensagem (broadcast, etc)
					if msg.IpDestino == ipHost { // usa a mensagem recebida
						if msg.JumpsCount > 6 {
							fmt.Println("Mensagem descartada por ter ultrapassado o limite de saltos")
							continue
						} else {
							fmt.Println("Repassado a mensagem para o próximo nó")
							msg.sendNextNode()
						}
					} else { // avalia o tipo e repassa a mensagem
						switch msg.Tipo {
						case "next": // repassa a mensagem
							fmt.Println("Repassado a mensagem para o próximo nó")

							msg.sendNextNode()
						default:

						}
					}
				} else {
					// tunnel connection. Aqui é onde algum nó faz uma conexão direta com o mestre

					// apos fazer o que tem q ser feito. Fechara conexão
					break
				}

			}
		}()
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

	cmd := exec.Command("clear") //Linux example, its tested
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	// definindo a porta do nó mestre
	ipIndexFile := flag.Int("fi", -1, "Indice o arquivo de ips")
	portaInicial := flag.String("p", "8080", "Porta inicial do nó mestre")
	flag.Parse()

	listIp := openFileAndGetIps("../ips")

	var err error
	// pega o ip da máquina sem a porta
	ipHost, err = getIpHost()
	errorHandler(err, "", true)

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	if *ipIndexFile == -1 {
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAuto(listIp, ipHost)
	} else {
		// atribui a porta
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAndHostManual(listIp, *ipIndexFile)
	}

	// Após a configuração inicial. Começa a receber as mensagens do anel
	go receiveMessageAnelListening()

	ipHostInitialConfig, _, _ := net.SplitHostPort(ipHost)

	// servidor tcp
	tcpListener, err := net.Listen("tcp", ipHostInitialConfig+":"+*portaInicial)
	errorHandler(err, "Erro ao criar servidor TCP: ", true)

	defer tcpListener.Close()

	// cria uma chave única SHA1
	privateKey = newHashSha1()

	// lida com conexoes de outros supernós
	fmt.Println("Aguardando supernós se conectarem...")

	// canais criados para controlar a conexão dos supernós
	ackChan := make(chan bool, 2)

	// canal para sinalizar que toda a rede está pronta
	finishChan := make(chan bool, 1)

	var conns []net.Conn

	for i := 0; i < 2; i++ {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão: ", false)

		fmt.Println("O super nó", i+1, " se conectou com sucesso!")
		conns = append(conns, conn)

		// envia a chave de identificação unica do nó mestre
		msg := newMensagem("chave", ipHost, conn.RemoteAddr().String(), []byte(privateKey), ipHost, 0)
		_, err = conn.Write(msg.toBytes())
		errorHandler(err, "Erro ao enviar a chave de identificação", false)

		go tcpHandleMessages(conn, ackChan, finishChan)
	}

	// Aguardar que ambos os nós se conectem
	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os super nós se conectaram com sucesso!")

		// envia confirmacao aos supernós
		for _, conn := range conns {

			// registros finalizados
			msg := newMensagem("ok", ipHost, conn.RemoteAddr().String(), []byte("ok"), ipHost, 0)

			_, err = conn.Write(msg.toBytes())
			errorHandler(err, "Erro ao enviar ok", false)
		}
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	if <-finishChan {
		// exemplos de mensagens que da a volta no anel duas vezes até chegar no nó mestre
		msg := newMensagem("next", ipHost, ipHost, []byte("MENSAGEM ENVIADA VIA ANEL"), ipHost, 0)
		err = msg.sendNextNode()
		errorHandler(err, "Erro ao enviar mensagem para o próximo nó: ", false)
	}

	select {}

}
