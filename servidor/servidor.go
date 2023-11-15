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
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HostAnel struct {
	IDHost string
	IPHost string
}

var tabelasDeRoteamentoSupers []HostAnel
var tabelasDeRoteamentoServidores []HostAnel

var mutexTabelasDeSupers = sync.Mutex{}
var mutexTabelasDeServ = sync.Mutex{}

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

func newAck(ipDestino string) *Mensagem {
	return &Mensagem{"ack", ipHost, ipDestino, []byte(""), ipHost, 0}
}

func (m *Mensagem) toString() string {
	return fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, m.JumpsCount)
}
func exitRing() {

	//informa um superno
	fmt.Println(tabelasDeRoteamentoSupers)
	destino := tabelasDeRoteamentoSupers[0].IPHost

	m := newMensagem("ServidorSairAnel", ipHost, destino, []byte(privateKey), ipHost, 0)
	m.sendNextNode()
	//espera  por um tempo
	time.Sleep(1 * time.Second)

	//informa antecessor e sucessor
	m = newMensagem("AtualizaProximo", ipHost, ipPrevNode, []byte(ipNextNode), ipHost, 0)
	m.sendPrevNode()
	m = newMensagem("AtualizaAnt", ipHost, ipNextNode, []byte(ipPrevNode), ipHost, 0)
	m.sendNextNode()
	time.Sleep(1 * time.Second)

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
	tcpListener, err := net.Listen("tcp", ipHost)

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

					case "InitListSuper":
						fmt.Println("Mensagem recebida, do superno: ", msg.toString())

						mutexTabelasDeSupers.Lock()

						err := json.Unmarshal(msg.Conteudo, &tabelasDeRoteamentoSupers)
						if err != nil {
							fmt.Println("Erro ao converter roteamento:", err)
						}

						mutexTabelasDeSupers.Unlock()

					case "InitListServ":
						mutexTabelasDeServ.Lock()
						err := json.Unmarshal(msg.Conteudo, &tabelasDeRoteamentoServidores)
						if err != nil {
							fmt.Println("Erro ao converter roteamento:", err)
						}

						mutexTabelasDeServ.Unlock()

					case "AtualizaProximo":
						closeNextNode()
						ipNextNode = string(msg.Conteudo)
						connectNextNode()
						conn.Write(newAck(conn.RemoteAddr().String()).toBytes())
					case "AtualizaAnt":
						closePrevNode()
						ipPrevNode = string(msg.Conteudo)
						connectPrevNode()
						conn.Write(newAck(conn.RemoteAddr().String()).toBytes())
					case "AtualizarListaServerSaida":
						var tabelaAnelAux []HostAnel

						err := json.Unmarshal(msg.Conteudo, &tabelaAnelAux)
						if err != nil {
							fmt.Println("erro de decodificar saida servidor")
						}
						mutexTabelasDeServ.Lock()
						aux := tabelasDeRoteamentoServidores
						for i, p := range tabelasDeRoteamentoServidores {
							if p.IDHost == string(msg.Conteudo) {
								aux = append(aux[:i], aux[i+1:]...)
								tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores[:i], tabelasDeRoteamentoServidores[i+1:]...)
							}

						}
						mutexTabelasDeServ.Unlock()
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
					default:
						fmt.Println("mensagem invalida")
					}
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
	ipConect := flag.String("c", "", "<ip>:<porta> de um superno. ")
	portaInicial := flag.String("p", "40503", "Porta inicial do nó mestre")
	flag.Parse()

	listIp := openFileAndGetIps("../ips")

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	var err error
	ipHost, err = getIpHost()
	errorHandler(err, "Erro ao obter endereço IP da máquina: ", true)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	if *ipConect == "" { //nesse caso o servidor é da configuração inicial
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
	} else {
		ipHost = ipHost + ":" + *portaInicial
		go receiveMessageAnelListening()
		joinRing(*ipConect)

		// recebe mensagens do anel

	}
	go func() {
		for sig := range c {
			fmt.Println(sig.String())
			fmt.Println("saindo do anel")
			exitRing()
			fmt.Println("saiu do anel")
			os.Exit(0)
		}
	}()
	// Wait forever
	select {}
}

func joinRing(ipSuper string) {
	//conecta solicita conexão super nó
	buffer := make([]byte, 4000)

	conn, err := net.Dial("tcp", ipSuper)

	//utiliza o ip e porta da primeira conexão para a conexão em anel

	//	defer conn.Close()
	ipNextNode = ipSuper
	if err != nil {
		errorHandler(err, "Erro ao conectar ao servidor:", true)
	}
	h := HostAnel{IPHost: ipHost, IDHost: privateKey}
	myhost, _ := json.Marshal(h)
	//envia mensagem de solicitação de conexão
	m := newMensagem("NovoServidor", ipHost, ipSuper, []byte(myhost), ipHost, 0)

	//m.enviarMensagemNext("NovoServidor")
	conn.Write(m.toBytes())

	// ack
	n, _ := conn.Read(buffer)

	buffer = buffer[:n]
	m, _ = splitMensagem(string(buffer))
	fmt.Println("Mensagem recebida, do superno: ", m.toString())

	ipPrevNode = string(m.Conteudo)

	fmt.Printf("guardo anterior [%s]\n", ipPrevNode)
	//enviar  ack

	ipPrevNode = strings.TrimSpace(ipPrevNode)
	fmt.Println("passou do ack")
	//enviar mensagem para o anterior atualizar o proximo ip

	conn2, err1 := net.Dial("tcp", ipPrevNode)

	if err1 != nil {
		fmt.Println("Erro ao conectar ao servidor:", err1)
		errorHandler(err1, "Erro ao conectar ao servidor:", true)
	}

	m = newMensagem("AtualizaProximo", ipHost, ipPrevNode, []byte(ipHost), ipHost, 0)

	conn2.Write(m.toBytes())
	n, err = conn2.Read(buffer)

	m, _ = splitMensagem(string(buffer[:n]))

	if !strings.EqualFold(m.Tipo, "ACK") {
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println("Erro ao atualizar anterior")
	} else {
		fmt.Println("anterior atualizado com sucesso")
	}
	msg := newAck(conn.RemoteAddr().String())

	conn.Write(msg.toBytes())
	//inicializa tabelas

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
		err = json.Unmarshal(msg.Conteudo, &tabelasDeRoteamentoSupers)
		errorHandler(err, "Erro ao converter tabela de roteamento:", true)

		for _, supers := range tabelasDeRoteamentoSupers {
			fmt.Println(supers.IDHost, " --- ", supers.IPHost)
		}
	}
}
