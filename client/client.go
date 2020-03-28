package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
)

const (
	maxLength = 257
	// NegotiationResponse
	versionLoc = 0
	methodLoc  = 1
)

type (
	// NegotiationRequest version numberofMethods methods
	NegotiationRequest struct {
		Ver      uint8
		Nmethods uint8
		Methods  []uint8
	}
	// NegotiationResponse version methods
	NegotiationResponse struct {
		Ver    uint8
		Method uint8
	}
	// Request version command fix type addresss port
	Request struct {
		Ver  uint8
		Com  uint8
		Fix  uint8
		Type uint8
		Addr []uint8
		port []uint8
	}
)

func setSystemProxy(listenAddr string) {
	cmd := exec.Command("./sysproxy.exe", "global", listenAddr)
	if err := cmd.Run(); err != nil {
		log.Println("[set systemProxy Error]", err)
	}
	fmt.Printf("successfully set systemProxy")
}

func send(ssServer *net.TCPConn, bytes []byte) {
	if _, err := ssServer.Write(bytes); err != nil {
		log.Println("[sendMessage Error]", err)
		return
	}
	log.Println("successfully sended")

}

func negotiation(ssServer *net.TCPConn, message *NegotiationRequest) {
	buf := make([]byte, 0)
	buf = append(buf, byte(message.Ver))
	buf = append(buf, byte(message.Nmethods))
	for i := uint8(0); i < message.Nmethods; i++ {
		buf = append(buf, message.Methods[i])
	}
	send(ssServer, buf)
}

func decodeNegotiationResponse(ssServer *net.TCPConn) (message *NegotiationResponse) {
	buf := make([]byte, maxLength)

	_, err := ssServer.Read(buf)
	if err != nil {
		log.Fatal(err)
	}

	message = new(NegotiationResponse)
	version := uint8(buf[versionLoc])
	if version != 5 {
		log.Println("NOT a Socks5 message")
		return
	}
	method := uint8(buf[methodLoc])

	message.Ver = version
	message.Method = method

	return
}

// func relayNegotiationRequest(client *net.TCPConn, message *NegotiationRequest) {
// 	buf := make([]byte, 0)
// 	send(client, buf)
// }

// func handleLocalClientRequest(localClient *net.TCPConn, serverAddr string) {
// 	serverAddress, err := net.ResolveTCPAddr("tcp", serverAddr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	ssServer, err := net.DialTCP("tcp", nil, serverAddress)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// set message data
// 	negotiationRequest := new(NegotiationRequest)
// 	negotiationRequest.Ver = 5
// 	negotiationRequest.Methods = append(negotiationRequest.Methods, 0)
// 	negotiationRequest.Methods = append(negotiationRequest.Methods, 1)
// 	negotiationRequest.Nmethods = uint8(len(negotiationRequest.Methods))
// 	negotiation(ssServer, negotiationRequest)

// 	message := decodeNegotiationResponse(ssServer)

// 	if message.Method == 0 {
// 		log.Println("connect fail")
// 		return
// 	}

// 	defer ssServer.Close()

// 	defer localClient.Close()

// }

func netCopy(input, output *net.TCPConn) (err error) {
	buf := make([]byte, 8192)
	for {
		count, err := input.Read(buf)
		if err != nil {
			if err == io.EOF && count > 0 {
				output.Write(buf[:count])
			}
			break
		}
		if count > 0 {
			output.Write(buf[:count])
		}
	}
	return
}

func handleLocalClientRequest(localClient *net.TCPConn, serverAddr string) {
	serverAddress, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	ssServer, err := net.DialTCP("tcp", nil, serverAddress)
	if err != nil {
		log.Fatal(err)
	}

	go netCopy(localClient, ssServer)
	netCopy(ssServer, localClient)

	defer ssServer.Close()
	defer localClient.Close()

}
func main() {
	var listenAddr, serverAddr string
	flag.StringVar(&listenAddr, "c", "127.0.0.1:8488", "Input listen address(127.0.0.1:8488):")
	flag.StringVar(&serverAddr, "s", "127.0.0.1:8489", "Input server address(127.0.0.1:8489):")

	listenAddress, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.ListenTCP("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}

	for {
		localClient, err := listener.AcceptTCP()
		fmt.Println("comming client:", localClient.RemoteAddr())
		if err != nil {
			log.Fatal(err)
		}
		go handleLocalClientRequest(localClient, serverAddr)
	}

}
