package fakessh

import (
	"io"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"
)

// directTCP carries a direct-tcpip channel to the address it names.
func (s *Server) directTCP(nch ssh.NewChannel) {
	var d struct {
		Host     string
		Port     uint32
		OrigHost string
		OrigPort uint32
	}
	if err := ssh.Unmarshal(nch.ExtraData(), &d); err != nil {
		_ = nch.Reject(ssh.ConnectionFailed, "bad request")
		return
	}
	nc, err := net.Dial("tcp", net.JoinHostPort(d.Host, strconv.Itoa(int(d.Port))))
	if err != nil {
		_ = nch.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	ch, reqs, err := nch.Accept()
	if err != nil {
		_ = nc.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() {
		_, _ = io.Copy(nc, ch)
		_ = nc.Close()
	}()
	_, _ = io.Copy(ch, nc)
	_ = ch.Close()
	_ = nc.Close()
}
