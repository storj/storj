package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gortc/stun"
)

var (
	server = flag.String("server", "", "The address to run the stun server")
)

const (
	udp           = "udp4"
	pingMsg       = "ping"
	pongMsg       = "pong"
	timeoutMillis = 500
)

func main() {
	flag.Parse()

	srvAddr, err := net.ResolveUDPAddr(udp, *server)
	if err != nil {
		log.Fatalln("resolve serveraddr:", err)
	}

	conn, err := net.ListenUDP(udp, nil)
	if err != nil {
		log.Fatalln("dial:", err)
	}

	defer conn.Close()

	log.Printf("Listening on %s\n", conn.LocalAddr())

	var publicAddr stun.XORMappedAddress
	var peerAddr *net.UDPAddr

	messageChan := listen(conn)
	var peerAddrChan <-chan string

	keepalive := time.Tick(timeoutMillis * time.Millisecond)
	keepaliveMsg := pingMsg

	var quit <-chan time.Time

	gotPong := false
	sentPong := false

	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				return
			}

			switch {
			case string(message) == pingMsg:
				keepaliveMsg = pongMsg

			case string(message) == pongMsg:
				if !gotPong {
					log.Println("Received pong message.")
				}

				// One client may skip sending ping if it receives
				// a ping message before knowning the peer address.
				keepaliveMsg = pongMsg

				gotPong = true

			case stun.IsMessage(message):
				m := new(stun.Message)
				m.Raw = message
				err := m.Decode()
				if err != nil {
					log.Println("decode:", err)
					break
				}
				var xorAddr stun.XORMappedAddress
				if err := xorAddr.GetFrom(m); err != nil {
					log.Println("getFrom:", err)
					break
				}

				if publicAddr.String() != xorAddr.String() {
					log.Printf("My public address: %s\n", xorAddr)
					publicAddr = xorAddr

					peerAddrChan = getPeerAddr()
				}

			default:
				log.Fatalln("unknown message", message)
			}

		case peerStr := <-peerAddrChan:
			peerAddr, err = net.ResolveUDPAddr(udp, peerStr)
			if err != nil {
				log.Fatalln("resolve peeraddr:", err)
			}

		case <-keepalive:
			// Keep NAT binding alive using STUN server or the peer once it's known
			if peerAddr == nil {
				err = sendBindingRequest(conn, srvAddr)
			} else {
				err = sendStr(keepaliveMsg, conn, peerAddr)
				if keepaliveMsg == pongMsg {
					sentPong = true
				}
			}

			if err != nil {
				log.Fatalln("keepalive:", err)
			}

		case <-quit:
			conn.Close()
		}

		if quit == nil && gotPong && sentPong {
			log.Println("Success! Quitting in two seconds.")
			quit = time.After(2 * time.Second)
		}
	}
}

func getPeerAddr() <-chan string {
	result := make(chan string)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		log.Println("Enter remote peer address:")
		peer, _ := reader.ReadString('\n')
		result <- strings.Trim(peer, " \r\n")
	}()

	return result
}

func listen(conn *net.UDPConn) <-chan []byte {
	messages := make(chan []byte)
	go func() {
		for {
			buf := make([]byte, 1024)

			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				close(messages)
				return
			}
			buf = buf[:n]

			messages <- buf
		}
	}()
	return messages
}

func sendBindingRequest(conn *net.UDPConn, addr *net.UDPAddr) error {
	m := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	err := send(m.Raw, conn, addr)
	if err != nil {
		return fmt.Errorf("binding: %v", err)
	}

	return nil
}

func send(msg []byte, conn *net.UDPConn, addr *net.UDPAddr) error {
	_, err := conn.WriteToUDP(msg, addr)
	if err != nil {
		return fmt.Errorf("send: %v", err)
	}

	return nil
}

func sendStr(msg string, conn *net.UDPConn, addr *net.UDPAddr) error {
	return send([]byte(msg), conn, addr)
}
