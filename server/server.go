//+build ignore
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
)

const (
	maxLength = 8192
	// NegotiationMessage
	versionLoc  = 0
	nmethodsLoc = 1
	// Request
	cmdLoc      = 1
	rsvLoc      = 2
	atypeLoc    = 3
	typeConnect = 1
	typeIPv4    = uint8(1)
	typeDomain  = uint8(3)
	typeIPv6    = uint8(4)
	lenIPv4     = 4
	lenIPv6     = 16
	lenDmBase   = 3 + 1 + 1 + 2
)

type (
	// NegotiationMessage version numberofMethods methods
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
	// Request
	Request struct {
		Ver     uint8
		Cmd     uint8
		Rsv     uint8
		Atyp    uint8
		dstPort uint16
		address string
	}
)

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
func send(ssServer *net.TCPConn, bytes []byte) (err error) {
	_, err = ssServer.Write(bytes)
	if err != nil {
		return
	}
	return
}
func decodeNegotiationRequest(client *net.TCPConn) (message *NegotiationRequest, err error) {
	buf := make([]byte, maxLength)

	_, err = client.Read(buf)
	if err != nil {
		err = errors.New("Client Read err")
		return
	}
	message = new(NegotiationRequest)
	version := uint8(buf[versionLoc])
	if version != 5 {
		err = errors.New("NOT a Socks5 message")
		return
	}
	nMethods := uint8(buf[nmethodsLoc])
	if nMethods == 0 {
		err = errors.New("Have 0 methods")
		return
	}

	message.Ver = version
	message.Nmethods = nMethods
	for i := uint8(2); i < (2 + message.Nmethods); i++ {
		message.Methods = append(message.Methods, uint8(buf[i]))
	}
	return

}

func sendNegotiationResponse(client *net.TCPConn, message *NegotiationResponse) (err error) {
	buf := make([]byte, 0)
	buf = append(buf, byte(message.Ver))
	buf = append(buf, byte(message.Method))
	err = send(client, buf)
	if err != nil {
		return
	}
	return
}

func decodeRequest(client *net.TCPConn) (message *Request, err error) {
	buf := make([]byte, maxLength)
	_, err = client.Read(buf)

	if err != nil {
		return
	}

	message = new(Request)
	version := uint8(buf[versionLoc])
	if version != 5 {
		err = errors.New("NOT a socks5 request")
		return
	}
	message.Ver = version
	command := uint8(buf[cmdLoc])
	if command != typeConnect {
		err = errors.New("only CONNECT be able to accept")
		return
	}
	message.Cmd = command
	rsa := uint8(buf[rsvLoc])
	atyp := uint8(buf[atypeLoc])
	message.Rsv = rsa
	message.Atyp = atyp
	var addrEnd int
	if atyp == typeIPv4 {
		ipStart := atypeLoc + 1
		addrEnd = ipStart + lenIPv4
		message.address = net.IP(buf[ipStart:addrEnd]).String()
	} else if atyp == typeIPv6 {
		ipStart := atypeLoc + 1
		addrEnd = ipStart + lenIPv6
		message.address = net.IP(buf[ipStart:addrEnd]).String()
	} else if atyp == typeDomain {
		addLengthLoc := atypeLoc + 1
		domainLength := int(buf[addLengthLoc])
		domainStart := addLengthLoc + 1
		addrEnd = int(addLengthLoc) + domainLength + 1
		message.address = string(buf[domainStart:addrEnd])
	} else {
		err = fmt.Errorf("address type is Unknown: %d", atyp)
		return
	}
	message.dstPort = binary.BigEndian.Uint16(buf[addrEnd : addrEnd+2])
	return
}

func sendResponse(client *net.TCPConn) (err error) {
	err = send(client, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x80, 0x88})
	if err != nil {
		return
	}
	return
}

func callTarget(client *net.TCPConn, request *Request) {
	serverAddr := request.address + ":" + strconv.Itoa(int(request.dstPort))
	serverAddress, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		log.Println("[ResolveTCPAddr err]", err)
		return
	}
	targetServer, err := net.DialTCP("tcp", nil, serverAddress)
	if err != nil {
		log.Println("[DialTCP err]", err)
		return
	}

	log.Println("send to", request.address, ":", request.dstPort)
	go netCopy(client, targetServer)
	go netCopy(targetServer, client)
}
func handlClientRequest(client *net.TCPConn) {

	message, err := decodeNegotiationRequest(client)
	if err != nil {
		log.Println("[decodeNegotiationRequest err]", err)
		return
	}

	NegotiationResponse := new(NegotiationResponse)
	NegotiationResponse.Ver = message.Ver
	NegotiationResponse.Method = 0
	err = sendNegotiationResponse(client, NegotiationResponse)
	if err != nil {
		log.Println("[sendNegotiationResponse err]", err)
		return
	}

	request, err := decodeRequest(client)

	if err != nil {
		log.Println("[decodeRequest err]", err)
		return
	}

	err = sendResponse(client)
	if err != nil {
		log.Println("[sendResponse err]", err)
		return
	}

	if request.Atyp == typeDomain {
		addr, err := net.ResolveIPAddr("ip", request.address)
		if err != nil {
			err = errors.New("NOT a socks5 request")
			return
		}
		request.address = addr.String()
		callTarget(client, request)
	} else if request.Atyp == typeIPv6 || request.Atyp == typeIPv4 {
		callTarget(client, request)
	} else {
		log.Println("callTarget error, unknow type")
	}

}

func main() {
	if len(os.Args) < 2 {
		log.Println("you should iput port number like that xx.go :xxx")
		return
	}

	listenAddr := "127.0.0.1" + os.Args[1]

	fmt.Println(listenAddr)

	listenAddress, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		log.Println("ResolveTCPAddr Error]", err)
		return
	}

	listener, err := net.ListenTCP("tcp", listenAddress)
	if err != nil {
		log.Println("ListenTCP Error]", err)
		return
	}

	for {
		client, err := listener.AcceptTCP()

		if err != nil {
			log.Println("AcceptTCP Error]", err)
			return
		}
		go handlClientRequest(client)
	}
}
