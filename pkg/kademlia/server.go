package kademlia

import "net"

// StartServer initalizes a connection to accept mesages from other nodes on the overlay network
func StartServer() (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, err
	}

	return net.ListenUDP("udp", addr)

}
