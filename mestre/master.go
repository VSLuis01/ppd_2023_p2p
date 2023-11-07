package main

import (
	"bufio"
	"crypto/sha1"
	"flag"
	"fmt"
	"github.com/multiformats/go-multiaddr"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

type TabelaDeRoteamento map[string][]multiaddr.Multiaddr

var ipNextNo string
var ipPrevNo string
var ipHost string
var privateKey string

/*type mensagem struct {
	IpOrigem  string
	IpDestino string
	Conteudo  string
	IpAtual   string
}

func (m *mensagem) enviarMensagemNext(tipo string) {
	//envia mensagem para o proximo
	sendMessageNext([]byte(tipo + "#" + m.IpOrigem + "#" + m.IpDestino + "#" + m.Conteudo + "#" + ipHost))
}

func (m *mensagem) enviarMensagemAnt(tipo string) {
	//envia mensagem para o proximo
	sendMessageAnt([]byte(tipo + "#" + m.IpOrigem + "#" + m.IpDestino + "#" + m.Conteudo + "#" + ipHost))
}*/

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

func tcpHandleConnection(conn net.Conn, chaveDeConexao string, ackChan chan<- bool, i int) {
	defer conn.Close()

	_, err := conn.Write([]byte(chaveDeConexao))
	if err != nil {
		fmt.Println("Erro ao enviar a chave de identificação:", err)
		ackChan <- false
		return
	}

	fmt.Println("Chave de identificação enviada para o super nó ", i+1)
	ack := make([]byte, 3)
	_, err = conn.Read(ack)
	if err != nil {
		fmt.Println("Erro ao ler o ACK do cliente: ", i+1, err)
		ackChan <- false
		return
	}

	if string(ack) == "ACK" {
		fmt.Println("ACK recebido do super nó ", i+1, " . Conexão estabelecida.")
		ackChan <- true
		return
	} else {
		fmt.Println("ACK inválido. Ação apropriada aqui (reenviar a chave, encerrar a conexão, etc.).")
		ackChan <- false
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
				if m.IpAtual == ipNextNo {
					fmt.Println("Mensagem recebida do nó proximo: ", m.Conteudo, " tipo: ", tipo)
					sendMessageNext(buffer[:n])
				} else {
					fmt.Println("Mensagem recebida do nó anterior: ")
				}

			}
		}()
	}
}*/

func sendMessageNext(mensagem []byte) {
	conn, err := net.Dial("tcp", ipNextNo)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()

	_, err = conn.Write(mensagem)
	errorHandler(err, "Erro ao enviar mensagem", true)
}

func sendMessageAnt(mensagem []byte) {
	conn, err := net.Dial("tcp", ipPrevNo)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()

	_, err = conn.Write(mensagem)
	errorHandler(err, "Erro ao enviar mensagem", true)
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

func getIpHost() string {
	addrs, err := net.InterfaceAddrs()
	errorHandler(err, "Erro ao obter endereços da interface: ", true)

	for _, address := range addrs {
		ipNet, ok := address.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
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
	ipHost := getIpHost()

	fmt.Println(ipHost)

	if *ipIndexFile == -1 {
		ipNextNo, ipPrevNo, ipHost = getNextAndPrevAuto(listIp, ipHost)
	} else {
		// atribui a porta
		ipNextNo, ipPrevNo, ipHost = getNextAndPrevAndHostManual(listIp, *ipIndexFile)
	}

	fmt.Println("ipNextNo: ", ipNextNo)
	fmt.Println("ipPrevNo: ", ipPrevNo)
	fmt.Println("ipHost: ", ipHost)

	///baseado no arquivo, encontra o ipatual e define proximo e anterior

	//inicia recepçao de mensagens do anel
	/*go receiveMessageAnelListening(ipHost)

	m := mensagem{"iporigem", "ipdestino", "conteudo", "ipatual"}
	m.enviarMensagemAnt("teste")*/

	// servidor tcp
	tcpListener, err := net.Listen("tcp", ipHost)
	errorHandler(err, "Erro ao criar servidor TCP: ", true)

	defer func(tcpListener net.Listener) {
		err := tcpListener.Close()
		errorHandler(err, "Erro ao fechar servidor TCP: ", true)
	}(tcpListener)

	// cria uma chave única SHA1
	//privateKey := newHashSha1()

	// lida com conexoes de outros supernós
	fmt.Println("Aguardando supernós se conectarem...")

	// canais criados para controlar a conexão dos supernós
	ackChan := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		_, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão: ", false)

		fmt.Println("O super nó", i+1, " se conectou com sucesso!")
		//go tcpHandleConnection(conn, chaveDeConexao, ackChan, i)
	}

	// Aguardar que ambos os nós se conectem
	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os super nós se conectaram com sucesso!")
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	fmt.Println("Enviando mensagem de broadcast...")

	select {}

}
