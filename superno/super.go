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
	"sync"
	"time"
)

type HostAnel struct {
	IDHost string
	IPHost string
}

var tabelaRoteamentoSuperNos []HostAnel
var tabelasDeRoteamentoServidores []HostAnel

var mutexTabelasDeSupers = sync.Mutex{}
var mutexTabelasDeServ = sync.Mutex{}

var ipNextNode string
var connNextNode net.Conn = nil

var ipPrevNode string
var connPrevNode net.Conn = nil

var ipHost string
var privateKey string

var privateKeyMestre string

// struct da mensagemh
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
		fmt.Println(mensagem)
		fmt.Println(mensagemSplit)
		return nil, fmt.Errorf("mensagem inválida")
	}

}

func newMensagem(tipo string, IpOrigem string, IpDestino string, conteudo []byte, IpHost string, jumpsCount int) *Mensagem {
	return &Mensagem{tipo, IpOrigem, IpDestino, conteudo, IpHost, jumpsCount}
}

func newAck(ipDestino string) *Mensagem {
	return &Mensagem{"ack", ipHost, ipDestino, []byte(""), ipHost, 0}
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

// Função responsável para tratar de conexões diretas
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
			return
		}

		errorHandler(err, "Erro ao ler mensagem TCP: ", false)

		mensagem := make([]byte, msgLen)
		copy(mensagem, buffer[:msgLen])

		msg, _ := splitMensagem(string(mensagem))

		fmt.Printf("[%s] Enviou: %s - Tipo > %s\n", conn.RemoteAddr().String(), string(msg.Conteudo), msg.Tipo)

		if strings.EqualFold(msg.Tipo, "roteamento-supers") {
			// slice dos ips dos supernos
			bytesRoteamento, err := json.Marshal(tabelaRoteamentoSuperNos)
			errorHandler(err, "Erro ao serializar tabela de roteamento: ", false)

			msg = newMensagem("roteamento-supers", ipHost, msg.IpOrigem, bytesRoteamento, ipHost, 0)

			_, err = conn.Write(msg.toBytes())
			errorHandler(err, "Erro ao enviar tabela de roteamento: ", false)

			continue
		} else if strings.EqualFold(msg.Tipo, "chave") {

			chaveIp := strings.Split(string(msg.Conteudo), "/")

			// adiciona a chave do servidor

			tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores, HostAnel{chaveIp[0], chaveIp[1]})
			msg = newMensagem("ack", ipHost, msg.IpOrigem, []byte("ack"), ipHost, 0)

			_, err = conn.Write(msg.toBytes())
			errorHandler(err, "Erro ao enviar ACK: ", false)

			ackChan <- true
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
	privateKey = newHashSha1()

	ipIndexFile := flag.Int("fi", -1, "Indice o arquivo de ips")

	// porta utilizada para os servidores se conectarem
	portForServers := flag.String("p", "8001", "Porta destino")
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

	// canal para sinalizar quando o mestre terminar de configurar a rede
	finishMestreChan := make(chan bool, 1)

	// canal para sinalizar quando os servidores terminarem de se conectar
	finishServersChan := make(chan bool, 1)

	ipMestre := listIp[0]

	ipMestreInitialConfig, _, _ := net.SplitHostPort(ipMestre)

	ipMestreInitialConfig = ipMestreInitialConfig + ":45661"

	// conecta com o mestre
	mestreConn, err := net.Dial("tcp", ipMestreInitialConfig)

	defer mestreConn.Close()

	errorHandler(err, "Erro ao conectar ao mestre:", true)
	fmt.Println("Conexão TCP estabelecida com sucesso")

	go configNoMestre(mestreConn, finishMestreChan)

	// Construção da Rede a partir daqui (todos os super nós se conectaram)
	if <-finishMestreChan {
		handleServers(ipHost, *portForServers, finishServersChan)
	}

	if <-finishServersChan {
		// apos os servidores terminarem de se conectar, envia ao mestre que a rede está pronta
		msg := newMensagem("ok", ipHost, mestreConn.RemoteAddr().String(), []byte("ok"), ipHost, 0)

		mestreConn.Write(msg.toBytes())

		// não precisa mais se conectar diretamento com o mestre. Mensagens via anel agora
		mestreConn.Close()
	}

	// Wait forever
	select {}
}

func configNoMestre(mestreConn net.Conn, finish chan<- bool) {

	chaveMestre := make([]byte, 1024)

	// recebe a chave de identificação do no mestre. ver o que fazer com isso
	lenMsg, err := mestreConn.Read(chaveMestre)
	errorHandler(err, "Erro ao receber chave do mestre:", false)

	chaveMestre = chaveMestre[:lenMsg]

	msg, _ := splitMensagem(string(chaveMestre))

	privateKeyMestre = string(msg.Conteudo)
	//go tcpHandleIncomingMessages(mestreConn)
	fmt.Println("Chave recebida do mestre: ", privateKeyMestre)

	fmt.Println("Enviando ACK para o nó mestre...")

	// envia um ACK com o IpHost real para a tabela de roteamento
	msg = newMensagem("ack", ipHost, mestreConn.RemoteAddr().String(), []byte(privateKey+"/"+ipHost), ipHost, 0)

	_, err = mestreConn.Write(msg.toBytes())
	errorHandler(err, "Erro ao enviar ACK:", true)

	// esperando todos so super nos se registrarem
	fmt.Println()
	fmt.Println("Aguardando supernós se registrarem...")

	confirmacao := make([]byte, 2048)
	msgLen, err := mestreConn.Read(confirmacao)

	errorHandler(err, "Erro ao receber confirmação do mestre:", true)
	confirmacao = confirmacao[:msgLen]

	msg, _ = splitMensagem(string(confirmacao))

	// registros dos super nós finalizados
	if msg.Tipo == "ok" {
		// solicitando tabela de roteamento
		fmt.Println("Solicitando tabela de roteamento ao mestre...")

		msg = newMensagem("roteamento-supers", ipHost, mestreConn.RemoteAddr().String(), []byte(""), ipHost, 0)

		// solicita a tabela de roteamento
		_, err = mestreConn.Write(msg.toBytes())
		errorHandler(err, "Erro ao solicitar tabela de roteamento:", true)

		// recebe a tabela de roteamento
		tabelaRoteamento := make([]byte, 2048)

		msgLen, err = mestreConn.Read(tabelaRoteamento)
		errorHandler(err, "Erro ao receber tabela de roteamento:", false)

		tabelaRoteamento = tabelaRoteamento[:msgLen]

		msg, _ = splitMensagem(string(tabelaRoteamento))

		fmt.Println("Tabela de roteamento recebida do mestre: ")
		err = json.Unmarshal(msg.Conteudo, &tabelaRoteamentoSuperNos)
		errorHandler(err, "Erro ao converter tabela de roteamento:", true)

		for _, supers := range tabelaRoteamentoSuperNos {
			fmt.Println(supers)
		}

		finish <- true
	} else {
		finish <- false
	}
}

func handleServers(ip string, portForServers string, finishServersChan chan<- bool) {
	ackChan := make(chan bool, 2)

	ip, _, _ = net.SplitHostPort(ip)

	ip = ip + ":" + portForServers

	tcpListener, err := net.Listen("tcp", ip)
	errorHandler(err, "Erro ao criar servidor TCP: ", false)
	if err != nil {
		return
	}

	defer tcpListener.Close()

	fmt.Println("Aguardando conexões dos servidores...")

	for i := 0; i < 2; i++ {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão:", false)
		fmt.Println("O servidor ", i+1, " se conectou...")

		go tcpHandleMessages(conn, ackChan)
	}

	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os servidores se conectaram com sucesso!")

		fmt.Println("Chaves dos servidores: ")

		for _, servidore := range tabelasDeRoteamentoServidores {
			fmt.Println("Chave: ", servidore.IDHost, " IP: ", servidore.IPHost)
		}

		finishServersChan <- true
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	return
}

// Receber conexoes da rede em anel
func receiveMessageAnelListening() {
	//tcpAddrIpHost, err := net.ResolveTCPAddr("tcp", ipHost)

	fmt.Println(ipHost)
	tcpListener, err := net.Listen("tcp", ipHost)

	defer tcpListener.Close()
	if err != nil {
		errorHandler(err, "Erro ao iniciar servidor TCP do anel: ", false)
		return
	}

	fmt.Println("Servidor TCP do anel iniciado...")
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

				if msg.IpOrigem == ipHost {
					continue
				}

				if msg.IpDestino != ipHost {
					if msg.IpAtual == ipNextNode {
						msg.sendPrevNode()
					} else if msg.IpAtual == ipPrevNode {
						msg.sendNextNode()
					} else {
						// tunel
					}
				} else {
					switch msg.Tipo {
					case "NovoServidor":
						var myp HostAnel
						err := json.Unmarshal(msg.Conteudo, &myp)
						//myp := HostAnel{IDHost: string(msg.Conteudo), IPHost: conn.RemoteAddr().String()}
						if err != nil {
							fmt.Println("erro de decodificar novo servidor")
						}

						mutexTabelasDeServ.Lock()
						tabelaAux := tabelasDeRoteamentoServidores
						existe := false

						for _, p := range tabelaAux {
							if p.IDHost == string(msg.Conteudo) {
								existe = true
							}
						}

						if !existe {
							tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores, myp)
						} else {
							fmt.Println("Servidor já registrado")
						}

						mutexTabelasDeServ.Unlock()

						fmt.Println("Novo servidor registrado: ", string(msg.Conteudo), " ; ", conn.RemoteAddr().String())

						newMsg := newMensagem("", ipHost, conn.RemoteAddr().String(), []byte(ipPrevNode), ipHost, 0)
						conn.Write(newMsg.toBytes())

						n, _ := conn.Read(buffer)
						buffer = buffer[:n]
						msg, _ = splitMensagem(string(buffer))
						if msg.Tipo == "ack" {
							fmt.Println("ACK recebido")
							time.Sleep(1 * time.Second)

							ipPrevNode = conn.RemoteAddr().String()
							byteTabelaServidores, _ := json.Marshal(tabelasDeRoteamentoServidores)
							fmt.Println(tabelasDeRoteamentoServidores)
							for _, p := range tabelasDeRoteamentoServidores {
								fmt.Println("Enviando tabela para: ", p.IPHost)
								mensagemEnvio := newMensagem("AtualizarListaServer", ipHost, p.IPHost, byteTabelaServidores, ipHost, 0)
								fmt.Println(mensagemEnvio.toString())
								mensagemEnvio.sendNextNode()
								time.Sleep(2 * time.Second)
							}

							fmt.Println(tabelaRoteamentoSuperNos)
							for _, p := range tabelaRoteamentoSuperNos {
								fmt.Println("Enviando tabela para: ", p.IPHost)
								mensagemEnvio := newMensagem("AtualizarListaServer", ipHost, p.IPHost, byteTabelaServidores, ipHost, 0)
								fmt.Println(mensagemEnvio.toString())
								mensagemEnvio.sendNextNode()
								time.Sleep(2 * time.Second)
							}

						}

						conn.Write(newAck(conn.RemoteAddr().String()).toBytes())
					case "AtualizarListaServer":

						var tabelaAnelAux []HostAnel
						var tabelaAnelAux2 []HostAnel
						err := json.Unmarshal(msg.Conteudo, &tabelaAnelAux)
						errorHandler(err, "Erro ao desconverter roteamento:", false)

						mutexTabelasDeServ.Lock()
						for _, p := range tabelaAnelAux { //laço para evitar itens duplicados
							existe := false
							for _, p2 := range tabelasDeRoteamentoServidores {

								if p.IDHost == p2.IDHost {
									existe = true

								}

							}
							if !existe {
								tabelaAnelAux2 = append(tabelaAnelAux2, p)
							}
						}
						tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores, tabelaAnelAux2...)
						mutexTabelasDeServ.Unlock()
					case "AtualizaProximo":
						ipNextNode = string(msg.Conteudo)
						conn.Write(newAck(conn.RemoteAddr().String()).toBytes())
					default:
						fmt.Println("mensagem invalida")
					}
				}

			}
		}()
	}
}
