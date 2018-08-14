package operations

import (
	"bytes"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// RemoteRun executes remote command
func RemoteRun(user string, addr string, port int, sshKey []byte, cmd string) (string, string, error) {
	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		return "", "error in ssh.ParsePrivateKey()", err
	}

	// Authentication
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error { return nil },
	}
	// Connect
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", addr, port), config)
	if err != nil {
		return "", "error in ssh.Dial()", err
	}
	// Create a session. It is one session per command.
	session, err := client.NewSession()
	if err != nil {
		return "", "error in NewSession()", err
	}
	defer session.Close()
	var bOut, bErr bytes.Buffer
	session.Stdout = &bOut // get output
	session.Stderr = &bErr // get error

	err = session.Run(cmd)
	return bOut.String(), bErr.String(), err
}
