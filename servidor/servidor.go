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

var tabelasDeRoteamento []string

var ipNextNode string
var connNextNode net.Conn = nil

var ipPrevNode string
var connPrevNode net.Conn = nil

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

	fmt.Println("Servidor TCP do anel iniciado com sucesso")
	for {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão TCP: ", false)

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

				msg, _ := splitMensagem(string(mensagem))

				//separa de quem veio a mensagem
				if msg.IpAtual == ipNextNode {
					// aqui vai a logica de tratamento da mensagem (broadcast, etc)

					if msg.IpDestino == ipHost { // usa a mensagem recebida
						if msg.JumpsCount > 6 {
							fmt.Println("Mensagem descartada por ter ultrapassado o limite de saltos")
							continue
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
					// tunnel connection. Aqui é onde algum nó faz uma conexão direta com o servidor

					// apos fazer o que tem q ser feito. Fechara conexão
					break
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

	cmd := exec.Command("clear") //Linux example, its tested
	cmd.Stdout = os.Stdout
	cmd.Run()
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

	// recebe mensagens do anel
	go receiveMessageAnelListening()

	fmt.Println("Se conectando a um super nó...")

	//por padrao o endereço do superno de configuração inicial é o segundo elemento da lista de ips
	ipSuperNo, _, _ := net.SplitHostPort(listIp[1])

	ipSuperNo = ipSuperNo + ":" + *portaSuperNo

	conn, err := net.Dial("tcp", ipSuperNo)
	errorHandler(err, "Erro ao conectar ao super nó:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")

	privateKey = newHashSha1()

	// configuração inicial com o supernó
	tcpConfigSuperNo(conn)

	// Wait forever
	select {}
}

// Conexões diretas com o nó servidor
func tcpConfigSuperNo(conn net.Conn) {
	defer conn.Close()

	msg := newMensagem("chave", ipHost, conn.RemoteAddr().String(), []byte(privateKey+"/"+ipHost), ipHost, 0)

	_, err := conn.Write(msg.toBytes())
	errorHandler(err, "Erro ao enviar chave para o super nó: ", false)
	fmt.Println("Chave enviada para o super nó")

	buffer := make([]byte, 4000)
	msgLen, err := conn.Read(buffer)

	if err == io.EOF {
		fmt.Printf("[%s] A conexão foi fechada pelo nó.\n", conn.RemoteAddr().String())
		return
	}

	errorHandler(err, "Erro ao ler mensagem TCP: ", false)

	mensagem := make([]byte, msgLen)
	copy(mensagem, buffer[:msgLen])

	msg, _ = splitMensagem(string(mensagem))

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
		msg, _ = splitMensagem(string(buffer))

		fmt.Println("Tabela de roteamento recebida do super nó: ")
		err = json.Unmarshal(msg.Conteudo, &tabelasDeRoteamento)
		errorHandler(err, "Erro ao converter tabela de roteamento:", true)

		for _, supers := range tabelasDeRoteamento {
			fmt.Println(supers)
		}
	}
}
