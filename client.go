package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHClient is a wrapper around c/crypto/ssh.Client
type SSHClient struct {
	client *ssh.Client
}

// NewSSHClient creates new SSHClient
func NewSSHClient(keyPath, user, host string, port int) (*SSHClient, error) {
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}
	return &SSHClient{client: client}, nil
}

// Close closes connection to SSH server
func (sc *SSHClient) Close() error {
	return sc.client.Close()
}

// Command runs command on SSH server and returns output as a io.Reader
func (sc *SSHClient) Command(cmd string) (io.Reader, error) {
	fmt.Println(cmd)
	session, err := sc.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	out, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	writer := io.MultiWriter(os.Stdout, buf)
	go io.Copy(writer, out)
	outerr, err := session.StderrPipe()
	if err != nil {
		return nil, err
	}
	go io.Copy(os.Stderr, outerr)
	err = session.Run(cmd)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// Commandf is similar to Command, but accepts extra params as fmt.Printf
func (sc *SSHClient) Commandf(format string, params ...interface{}) (io.Reader, error) {
	command := fmt.Sprintf(format, params...)
	return sc.Command(command)
}

// InteractiveCommandf is similar to InteractiveCommand, but accepts extra params as fmt.Printf
func (sc *SSHClient) InteractiveCommandf(format string, params ...interface{}) error {
	command := fmt.Sprintf(format, params...)
	return sc.InteractiveCommand(command)
}

// InteractiveCommand runs command on server, looking for keywords in output and sends replies to stdin
func (sc *SSHClient) InteractiveCommand(cmd string) error {
	session, err := sc.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	out, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	in, err := session.StdinPipe()
	if err != nil {
		return err
	}
	go processOut(out, in)
	outerr, err := session.StderrPipe()
	if err != nil {
		return err
	}
	go processOut(outerr, in)
	err = session.Shell()
	if err != nil {
		return err
	}
	fmt.Fprintf(in, "%s\n", cmd)
	return session.Wait()
}

func processOut(out io.Reader, in io.WriteCloser) {
	var prompts = map[string]string{
		"phrase:":         password,
		"private/ca.key:": password,
		"removal:":        "yes",
		"[Easy-RSA CA]:":  "GoVPN-RSA CA",
	}
	var fullOutput string
	scanner := bufio.NewScanner(out)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		text := scanner.Text()
		fullOutput = fullOutput + text
		fmt.Print(text)
		for key, reply := range prompts {
			if strings.HasSuffix(fullOutput, key) {
				fmt.Fprintf(in, "%s\n", reply)
			}
		}
		if strings.HasSuffix(fullOutput, "Data Base Updated") {
			in.Close()
			fmt.Println()
			return
		}
	}
}
